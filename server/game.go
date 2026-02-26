package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Game configuration (configurable via CLI flags / config file)
// ---------------------------------------------------------------------------

type GameConfig struct {
	WorldSize      int     `json:"worldSize"`
	FoodCount      int     `json:"foodCount"`
	AICount        int     `json:"aiCount"`
	BaseSpeed      float64 `json:"baseSpeed"`
	BoostSpeed     float64 `json:"boostSpeed"`
	TurnSpeed      float64 `json:"turnSpeed"`
	MaxBoost       float64 `json:"maxBoost"`
	BoostDrain     float64 `json:"boostDrain"`
	BoostRegen     float64 `json:"boostRegen"`
	BaseSnakeLen   int     `json:"baseSnakeLen"`
	KillFoodCount  int     `json:"killFoodCount"`
	BoundaryMargin float64 `json:"boundaryMargin"`
	AIRespawnTicks int     `json:"aiRespawnTicks"`
}

func DefaultConfig() GameConfig {
	return GameConfig{
		WorldSize:      10000,
		FoodCount:      3000,
		AICount:        30,
		BaseSpeed:      3.2,
		BoostSpeed:     5.5,
		TurnSpeed:      0.08,
		MaxBoost:       100,
		BoostDrain:     0.6,
		BoostRegen:     0.15,
		BaseSnakeLen:   10,
		KillFoodCount:  8,
		BoundaryMargin: 50,
		AIRespawnTicks: 180,
	}
}

// ---------------------------------------------------------------------------
// Fixed constants (technical/network, not configurable)
// ---------------------------------------------------------------------------
const (
	HeadRadius    = 12.0
	BodyRadius    = 10.0
	FoodRadiusVal = 6.0
	FoodValueVal  = 1.0
	TickRate      = 60
	NetTickRate   = 2
	FoodSyncRate  = 9
	ViewDist      = 2500.0
	FoodViewDist  = 1200.0
	NumColors     = 12
	NumFoodColors = 12
)

var aiNames = [...]string{
	"Viper", "Cobra", "Mamba", "Python", "Anaconda",
	"Rattler", "Boa", "Adder", "Asp", "Krait",
	"Taipan", "Coral", "Sidewinder", "Copperhead", "King",
	"Noodle", "Slinky", "Wiggles", "Scales", "Slithers",
	"Fangs", "Hissy", "Sssnake", "Danger", "Nope Rope",
}

var aiIDCounter int64

func nextAIID() int {
	return -int(atomic.AddInt64(&aiIDCounter, 1))
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type Vec2 struct{ X, Y float64 }

type Snake struct {
	Name        string
	Segments    []Vec2
	Angle       float64
	TargetAngle float64
	Speed       float64
	ColorIdx    int
	IsAI        bool
	PlayerID    int // -1 for AI
	Score       int
	TargetLen   int
	Boost       float64
	IsBoosting  bool
	Alive       bool
	InvTimer    int
	RespawnTmr  int // AI-only: frames until respawn

	AIState       string
	AIStateTimer  int
	AITargetAngle float64
}

type Food struct {
	X, Y     float64
	ColorIdx int
	Radius   float64
	Value    float64
}

type InputMsg struct {
	PlayerID int
	Angle    float64
	Boost    bool
}

type StatsSnapshot struct {
	Uptime         string             `json:"uptime"`
	UptimeSec      int64              `json:"uptimeSec"`
	TotalJoins     int64              `json:"totalJoins"`
	TotalLeaves    int64              `json:"totalLeaves"`
	TotalKills     int64              `json:"totalKills"`
	PeakPlayers    int                `json:"peakPlayers"`
	CurrentPlayers int                `json:"currentPlayers"`
	AICount        int                `json:"aiCount"`
	FoodCount      int                `json:"foodCount"`
	AvgTickMs      float64            `json:"avgTickMs"`
	MaxTickMs      float64            `json:"maxTickMs"`
	BandwidthKBps  float64            `json:"bandwidthKBps"`
	TotalBytesSent int64              `json:"totalBytesSent"`
	TotalBytesRecv int64              `json:"totalBytesRecv"`
	Frame          int                `json:"frame"`
	Leaderboard    []LeaderboardEntry `json:"leaderboard"`
}

type LeaderboardEntry struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	IsAI    bool   `json:"isAI"`
	IsAlive bool   `json:"alive"`
}

type Game struct {
	cfg     GameConfig
	snakes  []*Snake
	foods   []*Food
	players map[int]*Player

	frame   int
	netTick int

	inputCh   chan InputMsg
	joinCh    chan *Player
	leaveCh   chan int
	respawnCh chan int

	// Stats tracking
	startTime   time.Time
	totalJoins  int64
	totalLeaves int64
	totalKills  int64
	peakPlayers int

	// Tick performance
	tickDurations [60]time.Duration
	tickDurIdx    int
	maxTickMs     float64

	// Bandwidth tracking
	totalBytesSent int64
	totalBytesRecv int64 // atomic — written from readPump goroutines
	bwPerSec       [30]int64 // bytes-per-second ring buffer (last 30s)
	bwSecIdx       int
	bwAccum        int64 // bytes accumulated in the current second
	bwLastSec      int   // frame number of the last second boundary

	// Stats request channel (channel-of-channels for thread-safe reads)
	statsReqCh chan chan StatsSnapshot
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func distSq(x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	return dx*dx + dy*dy
}

func dist(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt(distSq(x1, y1, x2, y2))
}

func angleDiff(a, b float64) float64 {
	d := b - a
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	return d
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (g *Game) randWorldPos() Vec2 {
	ws := float64(g.cfg.WorldSize)
	return Vec2{
		X: 200 + rand.Float64()*(ws-400),
		Y: 200 + rand.Float64()*(ws-400),
	}
}

func headRadius(s *Snake) float64 {
	return HeadRadius + math.Min(float64(len(s.Segments))*0.03, 6)
}

func bodyRadius(s *Snake) float64 {
	return BodyRadius + math.Min(float64(len(s.Segments))*0.025, 5)
}

// ---------------------------------------------------------------------------
// Game constructor
// ---------------------------------------------------------------------------

func NewGame(cfg GameConfig) *Game {
	g := &Game{
		cfg:        cfg,
		players:    make(map[int]*Player),
		inputCh:    make(chan InputMsg, 2048),
		joinCh:     make(chan *Player, 32),
		leaveCh:    make(chan int, 32),
		respawnCh:  make(chan int, 32),
		startTime:  time.Now(),
		statsReqCh: make(chan chan StatsSnapshot, 4),
	}

	used := make(map[string]bool)
	for i := 0; i < cfg.AICount; i++ {
		name := aiNames[i%len(aiNames)]
		if used[name] {
			name = fmt.Sprintf("%s %d", aiNames[rand.Intn(len(aiNames))], i)
		}
		used[name] = true
		pos := g.randWorldPos()
		s := g.createSnake(name, pos.X, pos.Y, i%NumColors, true, nextAIID())
		extra := rand.Intn(40)
		s.TargetLen += extra
		s.Score += extra
		g.snakes = append(g.snakes, s)
	}

	for i := 0; i < cfg.FoodCount; i++ {
		g.foods = append(g.foods, g.newFood())
	}
	return g
}

// ---------------------------------------------------------------------------
// Snake
// ---------------------------------------------------------------------------

func (g *Game) createSnake(name string, x, y float64, colorIdx int, isAI bool, pid int) *Snake {
	angle := rand.Float64() * 2 * math.Pi
	segs := make([]Vec2, g.cfg.BaseSnakeLen)
	for i := range segs {
		segs[i] = Vec2{
			X: x - math.Cos(angle)*8*float64(i),
			Y: y - math.Sin(angle)*8*float64(i),
		}
	}
	return &Snake{
		Name: name, Segments: segs, Angle: angle, TargetAngle: angle,
		Speed: g.cfg.BaseSpeed, ColorIdx: colorIdx, IsAI: isAI, PlayerID: pid,
		TargetLen: g.cfg.BaseSnakeLen, Boost: g.cfg.MaxBoost, Alive: true, InvTimer: 120,
		AIState: "wander", AITargetAngle: angle,
	}
}

func (g *Game) growSnake(s *Snake, amt int) {
	s.TargetLen += amt
	s.Score += amt
}

func (g *Game) updateSnake(s *Snake) {
	if !s.Alive {
		return
	}
	if s.InvTimer > 0 {
		s.InvTimer--
	}

	diff := angleDiff(s.Angle, s.TargetAngle)
	s.Angle += clampF(diff, -g.cfg.TurnSpeed, g.cfg.TurnSpeed) * 1.8

	if s.IsBoosting && s.Boost > 0 && len(s.Segments) > 12 {
		s.Speed = g.cfg.BoostSpeed
		s.Boost -= g.cfg.BoostDrain
		if g.frame%8 == 0 && s.TargetLen > g.cfg.BaseSnakeLen {
			s.TargetLen--
			tail := s.Segments[len(s.Segments)-1]
			g.foods = append(g.foods, &Food{
				X: tail.X + rand.Float64()*20 - 10,
				Y: tail.Y + rand.Float64()*20 - 10,
				ColorIdx: rand.Intn(NumFoodColors),
				Radius:   FoodRadiusVal,
				Value:    FoodValueVal,
			})
		}
	} else {
		s.Speed = g.cfg.BaseSpeed
		s.IsBoosting = false
		if s.Boost < g.cfg.MaxBoost {
			s.Boost += g.cfg.BoostRegen
		}
	}

	head := s.Segments[0]
	newX := head.X + math.Cos(s.Angle)*s.Speed
	newY := head.Y + math.Sin(s.Angle)*s.Speed

	ws := float64(g.cfg.WorldSize)
	bm := g.cfg.BoundaryMargin
	if newX < bm || newX > ws-bm ||
		newY < bm || newY > ws-bm {
		if !s.IsAI {
			log.Printf("[DEATH] '%s' hit boundary (score: %d)", s.Name, s.Score)
			g.killSnake(s)
			return
		}
		s.TargetAngle = math.Atan2(ws/2-head.Y, ws/2-head.X)
		return
	}

	// Prepend new head
	s.Segments = append([]Vec2{{newX, newY}}, s.Segments...)
	for len(s.Segments) > s.TargetLen {
		s.Segments = s.Segments[:len(s.Segments)-1]
	}
}

func (g *Game) killSnake(s *Snake) {
	if !s.Alive {
		return
	}
	s.Alive = false

	step := len(s.Segments) / g.cfg.KillFoodCount
	if step < 1 {
		step = 1
	}
	for i := 0; i < len(s.Segments); i += step {
		seg := s.Segments[i]
		g.foods = append(g.foods, &Food{
			X: seg.X + rand.Float64()*30 - 15, Y: seg.Y + rand.Float64()*30 - 15,
			ColorIdx: rand.Intn(NumFoodColors),
			Radius:   7 + rand.Float64()*4,
			Value:    2 + rand.Float64()*3,
		})
	}

	if s.IsAI {
		s.RespawnTmr = g.cfg.AIRespawnTicks
	}
}

func (g *Game) respawnAI(s *Snake) {
	pos := g.randWorldPos()
	*s = *g.createSnake(s.Name, pos.X, pos.Y, rand.Intn(NumColors), true, nextAIID())
	extra := rand.Intn(40)
	s.TargetLen += extra
	s.Score += extra
}

// ---------------------------------------------------------------------------
// AI
// ---------------------------------------------------------------------------

func (g *Game) updateAI(s *Snake) {
	if !s.Alive || !s.IsAI {
		return
	}
	s.AIStateTimer--
	head := s.Segments[0]
	ws := float64(g.cfg.WorldSize)

	// Near boundary → flee
	if head.X < 300 || head.X > ws-300 || head.Y < 300 || head.Y > ws-300 {
		s.AIState = "flee"
		s.AIStateTimer = 30
	}

	// State transition
	if s.AIStateTimer <= 0 {
		r := rand.Float64()
		switch {
		case r < 0.5:
			s.AIState = "food"
			s.AIStateTimer = 60 + rand.Intn(120)
		case r < 0.8:
			s.AIState = "wander"
			s.AIStateTimer = 60 + rand.Intn(90)
			s.AITargetAngle = rand.Float64() * math.Pi * 2
		default:
			s.AIState = "hunt"
			s.AIStateTimer = 90 + rand.Intn(110)
		}
	}

	switch s.AIState {
	case "flee":
		s.TargetAngle = math.Atan2(ws/2-head.Y, ws/2-head.X) + rand.Float64()*0.6 - 0.3
		s.IsBoosting = true

	case "food":
		var closest *Food
		closestD := 400.0
		for _, f := range g.foods {
			d := dist(head.X, head.Y, f.X, f.Y)
			if d < closestD {
				closestD = d
				closest = f
			}
		}
		if closest != nil {
			s.TargetAngle = math.Atan2(closest.Y-head.Y, closest.X-head.X)
		} else {
			s.AIState = "wander"
			s.AIStateTimer = 60 + rand.Intn(60)
		}
		s.IsBoosting = false

	case "hunt":
		var target *Snake
		targetD := 500.0
		for _, o := range g.snakes {
			if o == s || !o.Alive || len(o.Segments) > int(float64(len(s.Segments))*1.5) {
				continue
			}
			d := dist(head.X, head.Y, o.Segments[0].X, o.Segments[0].Y)
			if d < targetD {
				targetD = d
				target = o
			}
		}
		if target != nil {
			th := target.Segments[0]
			px := th.X + math.Cos(target.Angle)*100
			py := th.Y + math.Sin(target.Angle)*100
			s.TargetAngle = math.Atan2(py-head.Y, px-head.X)
			s.IsBoosting = targetD < 200 && s.Boost > 30
		} else {
			s.AIState = "wander"
		}

	default: // wander
		if g.frame%60 == 0 {
			s.AITargetAngle += rand.Float64()*1.6 - 0.8
		}
		s.TargetAngle = s.AITargetAngle
		s.IsBoosting = false
	}

	// Collision avoidance
	for _, o := range g.snakes {
		if o == s || !o.Alive {
			continue
		}
		lim := len(o.Segments)
		if lim > 40 {
			lim = 40
		}
		for k := 0; k < lim; k += 2 {
			seg := o.Segments[k]
			d := dist(head.X, head.Y, seg.X, seg.Y)
			ad := bodyRadius(o) + headRadius(s) + 30
			if d < ad {
				s.TargetAngle = math.Atan2(head.Y-seg.Y, head.X-seg.X)
				s.IsBoosting = d < ad*0.6 && s.Boost > 20
				return // break both loops
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Food
// ---------------------------------------------------------------------------

func (g *Game) newFood() *Food {
	pos := g.randWorldPos()
	return &Food{
		X: pos.X, Y: pos.Y,
		ColorIdx: rand.Intn(NumFoodColors),
		Radius:   FoodRadiusVal,
		Value:    FoodValueVal,
	}
}

func (g *Game) checkFoodCollision(s *Snake) {
	if !s.Alive {
		return
	}
	head := s.Segments[0]
	hr := headRadius(s)

	n := len(g.foods)
	for i := n - 1; i >= 0; i-- {
		f := g.foods[i]
		if distSq(head.X, head.Y, f.X, f.Y) < (hr+f.Radius)*(hr+f.Radius) {
			g.growSnake(s, int(math.Round(f.Value)))
			// Remove food (swap with last)
			g.foods[i] = g.foods[len(g.foods)-1]
			g.foods = g.foods[:len(g.foods)-1]
		}
	}
}

// ---------------------------------------------------------------------------
// Snake-snake collision
// ---------------------------------------------------------------------------

func (g *Game) checkSnakeCollisions() {
	for _, s := range g.snakes {
		if !s.Alive || s.InvTimer > 0 {
			continue
		}
		head := s.Segments[0]
		hr := headRadius(s)

		for _, o := range g.snakes {
			if o == s || !o.Alive {
				continue
			}
			// Early-out: rough distance check against other snake's head
			oh := o.Segments[0]
			maxReach := float64(len(o.Segments)) * 8
			if distSq(head.X, head.Y, oh.X, oh.Y) > (maxReach+hr+50)*(maxReach+hr+50) {
				continue
			}

			br := bodyRadius(o)
			threshold := hr + br - 4
			thresholdSq := threshold * threshold

			for k := 5; k < len(o.Segments); k++ {
				seg := o.Segments[k]
				if distSq(head.X, head.Y, seg.X, seg.Y) < thresholdSq {
					g.totalKills++
					log.Printf("[KILL] '%s' killed by '%s' (score: %d)", s.Name, o.Name, s.Score)
					g.killSnake(s)
					g.growSnake(o, int(float64(len(s.Segments))*0.3))
					break
				}
			}
			if !s.Alive {
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Message processing (called from game loop only)
// ---------------------------------------------------------------------------

func (g *Game) drainMessages() {
	for {
		select {
		case msg := <-g.inputCh:
			if p, ok := g.players[msg.PlayerID]; ok && p.snake != nil && p.snake.Alive {
				p.snake.TargetAngle = msg.Angle
				p.snake.IsBoosting = msg.Boost
			}
		case p := <-g.joinCh:
			g.handleJoin(p)
		case id := <-g.leaveCh:
			g.handleLeave(id)
		case id := <-g.respawnCh:
			g.handleRespawn(id)
		case replyCh := <-g.statsReqCh:
			replyCh <- g.buildSnapshot()
		default:
			return
		}
	}
}

func (g *Game) handleJoin(p *Player) {
	// Remove one AI to make room
	for i, s := range g.snakes {
		if s.IsAI && s.Alive {
			g.snakes = append(g.snakes[:i], g.snakes[i+1:]...)
			break
		}
	}

	pos := g.randWorldPos()
	snake := g.createSnake(p.name, pos.X, pos.Y, rand.Intn(NumColors), false, p.id)
	p.snake = snake
	g.snakes = append(g.snakes, snake)
	g.players[p.id] = p
	g.totalJoins++
	current := len(g.players)
	if current > g.peakPlayers {
		g.peakPlayers = current
	}
	log.Printf("[JOIN] Player %d '%s' joined (players: %d, peak: %d)", p.id, p.name, current, g.peakPlayers)

	// Send full initial state
	data := g.serializeStateFor(p, true)
	select {
	case p.sendCh <- data:
	default:
	}
}

func (g *Game) handleLeave(id int) {
	p, ok := g.players[id]
	if !ok {
		return
	}
	g.totalLeaves++
	log.Printf("[LEAVE] Player %d '%s' left (players: %d)", id, p.name, len(g.players)-1)

	// Remove player's snake, replace with AI
	if p.snake != nil {
		for i, s := range g.snakes {
			if s == p.snake {
				g.snakes = append(g.snakes[:i], g.snakes[i+1:]...)
				break
			}
		}
		pos := g.randWorldPos()
		name := aiNames[rand.Intn(len(aiNames))]
		ai := g.createSnake(name, pos.X, pos.Y, rand.Intn(NumColors), true, nextAIID())
		extra := rand.Intn(40)
		ai.TargetLen += extra
		ai.Score += extra
		g.snakes = append(g.snakes, ai)
	}

	delete(g.players, id)
}

func (g *Game) handleRespawn(id int) {
	p, ok := g.players[id]
	if !ok || p.snake == nil || p.snake.Alive {
		return
	}

	// Remove dead snake
	for i, s := range g.snakes {
		if s == p.snake {
			g.snakes = append(g.snakes[:i], g.snakes[i+1:]...)
			break
		}
	}

	pos := g.randWorldPos()
	snake := g.createSnake(p.name, pos.X, pos.Y, rand.Intn(NumColors), false, p.id)
	p.snake = snake
	g.snakes = append(g.snakes, snake)
	// Invalidate metadata cache for this player's snake in all other players
	for _, other := range g.players {
		if other.knownSnakes != nil {
			delete(other.knownSnakes, p.id)
		}
	}
	log.Printf("[RESPAWN] Player %d '%s' respawned", id, p.name)
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

func (g *Game) buildSnapshot() StatsSnapshot {
	uptime := time.Since(g.startTime)

	var totalNs int64
	count := 0
	for _, d := range g.tickDurations {
		if d > 0 {
			totalNs += d.Nanoseconds()
			count++
		}
	}
	avgMs := 0.0
	if count > 0 {
		avgMs = float64(totalNs) / float64(count) / 1e6
	}

	// Compute average bandwidth (KB/s) from ring buffer
	var bwTotal int64
	bwCount := 0
	for _, b := range g.bwPerSec {
		if b > 0 {
			bwTotal += b
			bwCount++
		}
	}
	bwKBps := 0.0
	if bwCount > 0 {
		bwKBps = float64(bwTotal) / float64(bwCount) / 1024.0
	}

	aiCount := 0
	lb := make([]LeaderboardEntry, 0, len(g.snakes))
	for _, s := range g.snakes {
		if s.IsAI && s.Alive {
			aiCount++
		}
		if s.Alive {
			lb = append(lb, LeaderboardEntry{
				Name:    s.Name,
				Score:   s.Score,
				IsAI:    s.IsAI,
				IsAlive: s.Alive,
			})
		}
	}
	sort.Slice(lb, func(i, j int) bool { return lb[i].Score > lb[j].Score })
	if len(lb) > 20 {
		lb = lb[:20]
	}

	return StatsSnapshot{
		Uptime:         formatDuration(uptime),
		UptimeSec:      int64(uptime.Seconds()),
		TotalJoins:     g.totalJoins,
		TotalLeaves:    g.totalLeaves,
		TotalKills:     g.totalKills,
		PeakPlayers:    g.peakPlayers,
		CurrentPlayers: len(g.players),
		AICount:        aiCount,
		FoodCount:      len(g.foods),
		AvgTickMs:      math.Round(avgMs*100) / 100,
		MaxTickMs:      math.Round(g.maxTickMs*100) / 100,
		BandwidthKBps:  math.Round(bwKBps*100) / 100,
		TotalBytesSent: g.totalBytesSent,
		TotalBytesRecv: atomic.LoadInt64(&g.totalBytesRecv),
		Frame:          g.frame,
		Leaderboard:    lb,
	}
}

// ---------------------------------------------------------------------------
// Tick + Run
// ---------------------------------------------------------------------------

func (g *Game) tick() {
	start := time.Now()

	g.frame++
	g.drainMessages()

	for _, s := range g.snakes {
		if !s.Alive {
			if s.IsAI {
				s.RespawnTmr--
				if s.RespawnTmr <= 0 {
					g.respawnAI(s)
				}
			}
			continue
		}
		if s.IsAI {
			g.updateAI(s)
		}
		g.updateSnake(s)
		g.checkFoodCollision(s)
	}

	g.checkSnakeCollisions()

	for len(g.foods) < g.cfg.FoodCount {
		g.foods = append(g.foods, g.newFood())
	}

	if g.frame%NetTickRate == 0 {
		g.netTick++
		includeFood := g.netTick%FoodSyncRate == 0
		includeSummary := g.netTick%2 == 0
		g.broadcast(includeFood, includeSummary)
	}

	// Track tick performance
	elapsed := time.Since(start)
	g.tickDurations[g.tickDurIdx%len(g.tickDurations)] = elapsed
	g.tickDurIdx++
	ms := float64(elapsed.Nanoseconds()) / 1e6
	if ms > g.maxTickMs {
		g.maxTickMs = ms
	}

	// Flush bandwidth accumulator every second (every TickRate frames)
	if g.frame-g.bwLastSec >= TickRate {
		g.bwPerSec[g.bwSecIdx%len(g.bwPerSec)] = g.bwAccum
		g.bwSecIdx++
		g.bwAccum = 0
		g.bwLastSec = g.frame
	}

	// Periodic stats every ~30 seconds
	if g.frame%1800 == 0 {
		snap := g.buildSnapshot()
		log.Printf("[STATS] uptime=%s players=%d peak=%d ai=%d kills=%d food=%d avgTick=%.2fms maxTick=%.2fms bw=%.1fKB/s",
			snap.Uptime, snap.CurrentPlayers, snap.PeakPlayers, snap.AICount,
			snap.TotalKills, snap.FoodCount, snap.AvgTickMs, snap.MaxTickMs, snap.BandwidthKBps)
	}
}

func (g *Game) Run() {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()
	for range ticker.C {
		g.tick()
	}
}

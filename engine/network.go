package engine

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Player
// ---------------------------------------------------------------------------

type Player struct {
	id          int
	name        string
	conn        *websocket.Conn
	snake       *Snake
	sendCh      chan []byte
	done        chan struct{}
	knownSnakes map[int]bool // snake IDs whose metadata has been sent
}

var playerIDCounter int64

func nextPlayerID() int {
	return int(atomic.AddInt64(&playerIDCounter, 1))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ---------------------------------------------------------------------------
// WebSocket handler
// ---------------------------------------------------------------------------

func HandleWS(game *Game, w http.ResponseWriter, r *http.Request) {
	log.Printf("[WS] HTTP upgrade request from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	log.Printf("[WS] Upgrade complete for %s", r.RemoteAddr)

	id := nextPlayerID()
	p := &Player{
		id:          id,
		name:        fmt.Sprintf("Player %d", id),
		conn:        conn,
		sendCh:      make(chan []byte, 8),
		done:        make(chan struct{}),
		knownSnakes: make(map[int]bool),
	}

	// Send welcome (JSON, includes world size)
	welcome := fmt.Sprintf(`{"t":"welcome","pid":%d,"ws":%d,"v":"%s"}`, id, game.cfg.WorldSize, Version)
	conn.WriteMessage(websocket.TextMessage, []byte(welcome))
	log.Printf("[WS] Welcome sent to player %d (%s)", id, r.RemoteAddr)

	// Start writer
	go p.writePump()

	// Reader blocks here until disconnect
	p.readPump(game)

	// Cleanup
	close(p.done)
	game.leaveCh <- id
	conn.Close()
	log.Printf("Player %d (%s) disconnected", id, p.name)
}

// ---------------------------------------------------------------------------
// Read pump - one goroutine per player, reads client messages
// ---------------------------------------------------------------------------

func (p *Player) readPump(game *Game) {
	p.conn.SetReadLimit(512)
	p.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	p.conn.SetPongHandler(func(string) error {
		p.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		msgType, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		atomic.AddInt64(&game.totalBytesRecv, int64(len(data)))

		// Reset read deadline on any message
		p.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if msgType == websocket.TextMessage {
			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg["t"] {
			case "join":
				name, _ := msg["name"].(string)
				if name == "" {
					name = "Player"
				}
				if len(name) > 15 {
					name = name[:15]
				}
				p.name = name
				game.joinCh <- p
				log.Printf("Player %d joined as '%s'", p.id, p.name)
			case "respawn":
				game.respawnCh <- p.id
			}
		} else if msgType == websocket.BinaryMessage && len(data) == 4 && data[0] == 2 {
			// Input: type(1) + angle_int16(2) + boost(1)
			angle := float64(int16(binary.BigEndian.Uint16(data[1:3]))) / 10000.0
			boost := data[3]&1 != 0
			game.inputCh <- InputMsg{PlayerID: p.id, Angle: angle, Boost: boost}
		}
	}
}

// ---------------------------------------------------------------------------
// Write pump - one goroutine per player, sends messages to client
// ---------------------------------------------------------------------------

func (p *Player) writePump() {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case msg, ok := <-p.sendCh:
			if !ok {
				return
			}
			p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := p.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-p.done:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// State serialization (binary protocol - must match client exactly)
//
// Header: type(1)=1, flags(1), snakeCount(uint16 BE)
//   flags: bit0=hasFood, bit1=hasSummary
// Per snake:
//   playerId(int16 BE),
//   flags(uint8: bit0=alive, bit1=boosting, bit2=isPlayer, bit3=hasMeta),
//   [if hasMeta: nameLen(uint8), name[nameLen], colorIdx(uint8)],
//   score(uint16 BE), angle*10000(int16 BE), boost(uint8),
//   targetLen(uint16 BE), invTimer(uint8),
//   segCount(uint16 BE), segments[segCount * 4](uint16 x + uint16 y, BE) — every 3rd segment
// If hasFood:
//   foodCount(uint16 BE)
//   Per food(7 bytes): x(uint16), y(uint16), colorIdx(uint8),
//                      radius*10(uint8), value*10(uint8)
// If hasSummary (appended by broadcast):
//   summaryCount(uint16 BE)
//   Per alive snake: playerId(int16), headX(uint16), headY(uint16),
//                    score(uint16), colorIdx(uint8), nameLen(uint8), name[nameLen]
// ---------------------------------------------------------------------------

func (g *Game) serializeStateFor(p *Player, includeFood bool) []byte {
	// Determine visible snakes (viewport filtered)
	var visible []*Snake
	var cx, cy float64
	if p.snake != nil && len(p.snake.Segments) > 0 {
		cx = p.snake.Segments[0].X
		cy = p.snake.Segments[0].Y
	} else {
		cx = float64(g.cfg.WorldSize) / 2
		cy = float64(g.cfg.WorldSize) / 2
	}

	// Always include own snake
	if p.snake != nil {
		visible = append(visible, p.snake)
	}
	for _, s := range g.snakes {
		if s == p.snake {
			continue
		}
		if !s.Alive || len(s.Segments) == 0 {
			continue
		}
		sh := s.Segments[0]
		dx := math.Abs(sh.X - cx)
		dy := math.Abs(sh.Y - cy)
		if dx < ViewDist+1000 && dy < ViewDist+1000 {
			visible = append(visible, s)
		}
	}

	// Build hasMeta flags: true for snakes whose metadata hasn't been sent yet
	if p.knownSnakes == nil {
		p.knownSnakes = make(map[int]bool)
	}
	hasMeta := make([]bool, len(visible))
	newKnown := make(map[int]bool, len(visible))
	for i, s := range visible {
		if !p.knownSnakes[s.PlayerID] {
			hasMeta[i] = true
		}
		newKnown[s.PlayerID] = true
	}
	p.knownSnakes = newKnown

	// Determine visible food
	var visibleFood []*Food
	if includeFood {
		for _, f := range g.foods {
			if math.Abs(f.X-cx) < FoodViewDist && math.Abs(f.Y-cy) < FoodViewDist {
				visibleFood = append(visibleFood, f)
			}
		}
	}

	return serializeState(visible, hasMeta, visibleFood, includeFood)
}

func serializeState(snakes []*Snake, hasMeta []bool, foods []*Food, includeFood bool) []byte {
	// Calculate buffer size
	size := 4 // header
	for i, s := range snakes {
		segCount := (len(s.Segments) + 2) / 3 // ceil(n/3)
		// playerId(2) + flags(1) + score(2) + angle(2) + boost(1) + targetLen(2) + invTimer(1) + segCount(2) + segs
		perSnake := 2 + 1 + 2 + 2 + 1 + 2 + 1 + 2 + segCount*4
		if hasMeta == nil || hasMeta[i] {
			perSnake += 1 + len(s.Name) + 1 // nameLen + name + colorIdx
		}
		size += perSnake
	}
	if includeFood {
		size += 2 + len(foods)*7
	}

	buf := make([]byte, size)
	o := 0

	// Header
	buf[o] = 1 // type = state
	o++
	if includeFood {
		buf[o] = 1
	}
	o++
	binary.BigEndian.PutUint16(buf[o:], uint16(len(snakes)))
	o += 2

	// Snakes
	for i, s := range snakes {
		// PlayerId first
		binary.BigEndian.PutUint16(buf[o:], uint16(int16(s.PlayerID)))
		o += 2

		// Flags with hasMeta bit
		var flags byte
		if s.Alive {
			flags |= 1
		}
		if s.IsBoosting {
			flags |= 2
		}
		if !s.IsAI {
			flags |= 4
		}
		meta := hasMeta == nil || hasMeta[i]
		if meta {
			flags |= 8
		}
		buf[o] = flags
		o++

		// Conditional metadata
		if meta {
			nameBytes := []byte(s.Name)
			buf[o] = byte(len(nameBytes))
			o++
			copy(buf[o:], nameBytes)
			o += len(nameBytes)

			buf[o] = byte(s.ColorIdx)
			o++
		}

		score := s.Score
		if score > 65535 {
			score = 65535
		}
		binary.BigEndian.PutUint16(buf[o:], uint16(score))
		o += 2

		// Angle normalized to [-PI, PI]
		a := s.Angle
		for a > math.Pi {
			a -= 2 * math.Pi
		}
		for a < -math.Pi {
			a += 2 * math.Pi
		}
		binary.BigEndian.PutUint16(buf[o:], uint16(int16(math.Round(a*10000))))
		o += 2

		boost := int(math.Round(s.Boost))
		if boost < 0 {
			boost = 0
		}
		if boost > 255 {
			boost = 255
		}
		buf[o] = byte(boost)
		o++

		tl := s.TargetLen
		if tl > 65535 {
			tl = 65535
		}
		binary.BigEndian.PutUint16(buf[o:], uint16(tl))
		o += 2

		inv := s.InvTimer
		if inv > 255 {
			inv = 255
		}
		buf[o] = byte(inv)
		o++

		// Segments (every 3rd)
		segCount := (len(s.Segments) + 2) / 3
		binary.BigEndian.PutUint16(buf[o:], uint16(segCount))
		o += 2
		for j := 0; j < len(s.Segments); j += 3 {
			x := int(math.Round(s.Segments[j].X))
			y := int(math.Round(s.Segments[j].Y))
			if x < 0 {
				x = 0
			}
			if x > 65535 {
				x = 65535
			}
			if y < 0 {
				y = 0
			}
			if y > 65535 {
				y = 65535
			}
			binary.BigEndian.PutUint16(buf[o:], uint16(x))
			o += 2
			binary.BigEndian.PutUint16(buf[o:], uint16(y))
			o += 2
		}
	}

	// Food
	if includeFood {
		binary.BigEndian.PutUint16(buf[o:], uint16(len(foods)))
		o += 2
		for _, f := range foods {
			x := int(math.Round(f.X))
			y := int(math.Round(f.Y))
			if x < 0 {
				x = 0
			}
			if x > 65535 {
				x = 65535
			}
			if y < 0 {
				y = 0
			}
			if y > 65535 {
				y = 65535
			}
			binary.BigEndian.PutUint16(buf[o:], uint16(x))
			o += 2
			binary.BigEndian.PutUint16(buf[o:], uint16(y))
			o += 2
			buf[o] = byte(f.ColorIdx)
			o++
			r := int(math.Round(f.Radius * 10))
			if r > 255 {
				r = 255
			}
			buf[o] = byte(r)
			o++
			v := int(math.Round(f.Value * 10))
			if v > 255 {
				v = 255
			}
			buf[o] = byte(v)
			o++
		}
	}

	return buf[:o]
}

// ---------------------------------------------------------------------------
// Global summary (leaderboard + minimap for ALL alive snakes, not viewport-filtered)
// ---------------------------------------------------------------------------

func (g *Game) buildSummaryBytes() []byte {
	var alive []*Snake
	for _, s := range g.snakes {
		if s.Alive && len(s.Segments) > 0 {
			alive = append(alive, s)
		}
	}

	// Calculate size: 2 (count) + per snake: 2+2+2+2+1+1+nameLen
	size := 2
	for _, s := range alive {
		size += 2 + 2 + 2 + 2 + 1 + 1 + len(s.Name)
	}

	buf := make([]byte, size)
	o := 0
	binary.BigEndian.PutUint16(buf[o:], uint16(len(alive)))
	o += 2

	for _, s := range alive {
		binary.BigEndian.PutUint16(buf[o:], uint16(int16(s.PlayerID)))
		o += 2

		hx := int(math.Round(s.Segments[0].X))
		if hx < 0 {
			hx = 0
		}
		if hx > 65535 {
			hx = 65535
		}
		hy := int(math.Round(s.Segments[0].Y))
		if hy < 0 {
			hy = 0
		}
		if hy > 65535 {
			hy = 65535
		}
		binary.BigEndian.PutUint16(buf[o:], uint16(hx))
		o += 2
		binary.BigEndian.PutUint16(buf[o:], uint16(hy))
		o += 2

		score := s.Score
		if score > 65535 {
			score = 65535
		}
		binary.BigEndian.PutUint16(buf[o:], uint16(score))
		o += 2

		buf[o] = byte(s.ColorIdx)
		o++

		nameBytes := []byte(s.Name)
		buf[o] = byte(len(nameBytes))
		o++
		copy(buf[o:], nameBytes)
		o += len(nameBytes)
	}

	return buf[:o]
}

// ---------------------------------------------------------------------------
// Broadcast (called from game loop goroutine)
// ---------------------------------------------------------------------------

func (g *Game) broadcast(includeFood bool, includeSummary bool) {
	var summaryBytes []byte
	if includeSummary {
		summaryBytes = g.buildSummaryBytes()
	}

	for _, p := range g.players {
		if p.snake == nil {
			continue
		}
		oldKnown := p.knownSnakes
		data := g.serializeStateFor(p, includeFood)

		// Append global summary and set hasSummary flag (bit 1)
		if includeSummary && len(summaryBytes) > 0 {
			full := make([]byte, len(data)+len(summaryBytes))
			copy(full, data)
			copy(full[len(data):], summaryBytes)
			full[1] |= 2 // flags bit 1 = hasSummary
			data = full
		}

		n := int64(len(data))
		select {
		case p.sendCh <- data:
			g.totalBytesSent += n
			g.bwAccum += n
		default:
			// Buffer full, drop frame — restore knownSnakes so metadata is resent
			p.knownSnakes = oldKnown
		}
	}
}

// ---------------------------------------------------------------------------
// Stats API + Dashboard
// ---------------------------------------------------------------------------

func HandleStats(game *Game, w http.ResponseWriter, r *http.Request) {
	snap := game.GetStats()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(snap)
}

func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Schlangen.TV Dashboard</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
         background: #1a1a2e; color: #eee; padding: 20px; }
  h1 { background: linear-gradient(135deg, #e94560, #c23152); padding: 14px 24px;
       border-radius: 10px; margin-bottom: 24px; color: white; font-size: 22px;
       display: flex; align-items: center; justify-content: space-between; }
  h1 .dot { width: 10px; height: 10px; border-radius: 50%; background: #0f0;
            display: inline-block; margin-right: 8px; animation: pulse 2s infinite; }
  @keyframes pulse { 0%,100% { opacity:1; } 50% { opacity:0.4; } }
  h2 { margin-bottom: 12px; font-size: 16px; color: #aaa; text-transform: uppercase;
       letter-spacing: 1px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
          gap: 14px; margin-bottom: 28px; }
  .card { background: #16213e; border-radius: 10px; padding: 18px;
          border-left: 4px solid #0f3460; transition: transform 0.15s; }
  .card:hover { transform: translateY(-2px); }
  .card .label { font-size: 11px; text-transform: uppercase; color: #888;
                 letter-spacing: 0.5px; }
  .card .value { font-size: 32px; font-weight: bold; color: #e94560; margin-top: 4px;
                 font-variant-numeric: tabular-nums; }
  .card .unit { font-size: 13px; color: #666; }
  .card.perf { border-left-color: #00cc88; }
  .card.perf .value { color: #00cc88; }
  table { width: 100%; border-collapse: collapse; background: #16213e;
          border-radius: 10px; overflow: hidden; }
  th { background: #0f3460; padding: 10px 14px; text-align: left; font-size: 12px;
       text-transform: uppercase; letter-spacing: 0.5px; }
  td { padding: 9px 14px; border-bottom: 1px solid #1a1a2e; font-size: 14px; }
  tr:hover td { background: #1a1a3e; }
  .badge { color: white; padding: 2px 8px; border-radius: 4px; font-size: 11px;
           font-weight: 600; }
  .badge.ai { background: #533483; }
  .badge.player { background: #0f3460; }
  .rank { color: #666; font-weight: bold; }
  .status-bar { font-size: 11px; color: #555; margin-top: 16px; text-align: right; }
</style>
</head>
<body>
<h1><span><span class="dot"></span>Schlangen.TV Server <span id="version" style="font-size:13px;font-weight:normal;color:rgba(255,255,255,0.5)"></span></span><span id="uptime" style="font-size:14px;font-weight:normal;color:rgba(255,255,255,0.7)"></span></h1>
<div class="grid" id="cards"></div>
<h2>Leaderboard</h2>
<table>
  <thead><tr><th>#</th><th>Name</th><th>Score</th><th>Type</th></tr></thead>
  <tbody id="lb"></tbody>
</table>
<div class="status-bar" id="status">Connecting...</div>
<script>
function fmtBw(v) { return v >= 1024 ? (v/1024).toFixed(1)+'<span class="unit"> MB/s</span>' : v+'<span class="unit"> KB/s</span>'; }
function fmtBytes(v) {
  if (v >= 1073741824) return (v/1073741824).toFixed(2)+'<span class="unit"> GB</span>';
  if (v >= 1048576) return (v/1048576).toFixed(1)+'<span class="unit"> MB</span>';
  if (v >= 1024) return (v/1024).toFixed(1)+'<span class="unit"> KB</span>';
  return v+'<span class="unit"> B</span>';
}
const cardDefs = [
  {k:'currentPlayers', label:'Players Online', unit:''},
  {k:'peakPlayers',    label:'Peak Players',   unit:''},
  {k:'aiCount',        label:'AI Snakes',      unit:''},
  {k:'foodCount',      label:'Food Items',     unit:''},
  {k:'totalKills',     label:'Total Kills',    unit:''},
  {k:'totalJoins',     label:'Total Joins',    unit:''},
  {k:'totalLeaves',    label:'Total Leaves',   unit:''},
  {k:'avgTickMs',      label:'Avg Tick',       unit:'ms', perf:true},
  {k:'maxTickMs',      label:'Max Tick',       unit:'ms', perf:true},
  {k:'bandwidthKBps',  label:'Bandwidth Out',  unit:'KB/s', perf:true, fmt:fmtBw},
  {k:'totalBytesSent', label:'Total Sent',     unit:'', perf:true, fmt:fmtBytes},
  {k:'totalBytesRecv', label:'Total Received', unit:'', perf:true, fmt:fmtBytes},
  {k:'memAllocMB',     label:'Heap Memory',    unit:'MB', perf:true},
  {k:'memSysMB',       label:'System Memory',  unit:'MB', perf:true},
  {k:'numGoroutines',  label:'Goroutines',     unit:'',   perf:true},
  {k:'gcPauseMs',      label:'GC Pause',       unit:'ms', perf:true},
];
function render(d) {
  document.getElementById('uptime').textContent = d.uptime || '';
  if (d.version) document.getElementById('version').textContent = 'v' + d.version;
  let html = '';
  for (const c of cardDefs) {
    let v = d[c.k];
    if (v === undefined) v = '-';
    let valHtml = c.fmt ? c.fmt(v) : v+' <span class="unit">'+c.unit+'</span>';
    html += '<div class="card'+(c.perf?' perf':'')+'"><div class="label">'+c.label+'</div>'+
            '<div class="value">'+valHtml+'</div></div>';
  }
  document.getElementById('cards').innerHTML = html;
  let lb = '';
  if (d.leaderboard && d.leaderboard.length) {
    d.leaderboard.forEach(function(e, i) {
      let badge = e.isAI ? '<span class="badge ai">AI</span>'
                         : '<span class="badge player">Player</span>';
      lb += '<tr><td class="rank">'+(i+1)+'</td><td>'+esc(e.name)+'</td><td>'+e.score+'</td><td>'+badge+'</td></tr>';
    });
  } else {
    lb = '<tr><td colspan="4" style="color:#555;text-align:center">No snakes alive</td></tr>';
  }
  document.getElementById('lb').innerHTML = lb;
  document.getElementById('status').textContent = 'Last update: ' + new Date().toLocaleTimeString();
}
function esc(s) { let d=document.createElement('div'); d.textContent=s; return d.innerHTML; }
function poll() {
  fetch('/stats').then(r=>r.json()).then(render)
    .catch(e=>{ document.getElementById('status').textContent='Error: '+e; });
}
poll();
setInterval(poll, 1000);
</script>
</body>
</html>`

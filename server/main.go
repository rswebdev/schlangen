package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"snake.io/engine"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	configFile := flag.String("config", "", "Path to JSON config file")
	worldSize := flag.Int("world-size", 0, "World size (default 10000)")
	foodCount := flag.Int("food-count", 0, "Food item count (default 3000)")
	aiCount := flag.Int("ai-count", 0, "AI snake count (default 30)")
	baseSpeed := flag.Float64("base-speed", 0, "Base snake speed (default 3.2)")
	boostSpeed := flag.Float64("boost-speed", 0, "Boost speed (default 5.5)")
	turnSpeed := flag.Float64("turn-speed", 0, "Turn speed (default 0.08)")
	maxBoost := flag.Float64("max-boost", 0, "Max boost meter (default 100)")
	boostDrain := flag.Float64("boost-drain", 0, "Boost drain rate (default 0.6)")
	boostRegen := flag.Float64("boost-regen", 0, "Boost regen rate (default 0.15)")
	baseSnakeLen := flag.Int("base-snake-len", 0, "Base snake length (default 10)")
	killFoodCount := flag.Int("kill-food-count", 0, "Food dropped on kill (default 8)")
	boundaryMargin := flag.Float64("boundary-margin", 0, "Boundary margin (default 50)")
	aiRespawnTicks := flag.Int("ai-respawn-ticks", 0, "AI respawn delay in ticks (default 180)")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime)

	// Build config: defaults -> config file -> CLI overrides
	cfg := engine.DefaultConfig()

	if *configFile != "" {
		data, err := os.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("Failed to parse config file: %v", err)
		}
		log.Printf("Loaded config from %s", *configFile)
	}

	// CLI flag overrides (non-zero values override config file)
	if *worldSize > 0 {
		cfg.WorldSize = *worldSize
	}
	if *foodCount > 0 {
		cfg.FoodCount = *foodCount
	}
	if *aiCount > 0 {
		cfg.AICount = *aiCount
	}
	if *baseSpeed > 0 {
		cfg.BaseSpeed = *baseSpeed
	}
	if *boostSpeed > 0 {
		cfg.BoostSpeed = *boostSpeed
	}
	if *turnSpeed > 0 {
		cfg.TurnSpeed = *turnSpeed
	}
	if *maxBoost > 0 {
		cfg.MaxBoost = *maxBoost
	}
	if *boostDrain > 0 {
		cfg.BoostDrain = *boostDrain
	}
	if *boostRegen > 0 {
		cfg.BoostRegen = *boostRegen
	}
	if *baseSnakeLen > 0 {
		cfg.BaseSnakeLen = *baseSnakeLen
	}
	if *killFoodCount > 0 {
		cfg.KillFoodCount = *killFoodCount
	}
	if *boundaryMargin > 0 {
		cfg.BoundaryMargin = *boundaryMargin
	}
	if *aiRespawnTicks > 0 {
		cfg.AIRespawnTicks = *aiRespawnTicks
	}

	log.Printf("Config: worldSize=%d food=%d ai=%d speed=%.1f boost=%.1f",
		cfg.WorldSize, cfg.FoodCount, cfg.AICount, cfg.BaseSpeed, cfg.BoostSpeed)

	srv := engine.NewServer(cfg)
	log.Fatal(srv.ListenAndServe(*port))
}

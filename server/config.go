package main

import (
	"log"

	"github.com/spf13/viper"
)

// GameConfig is the full configuration served to the WASM client.
type GameConfig struct {
	Game   GameSettings             `json:"game"`
	Levels map[string]LevelSettings `json:"levels"`
}

type GameSettings struct {
	Duration               float64 `mapstructure:"duration"                 json:"duration"`
	BallSpeedupRate        float64 `mapstructure:"ball_speedup_rate"        json:"ball_speedup_rate"`
	PlayerSpeed            float64 `mapstructure:"player_speed"             json:"player_speed"`
	HitFreezeDuration      float64 `mapstructure:"hit_freeze_duration"      json:"hit_freeze_duration"`
	CountdownDigitDuration float64 `mapstructure:"countdown_digit_duration" json:"countdown_digit_duration"`
	GoDisplayDuration      float64 `mapstructure:"go_display_duration"      json:"go_display_duration"`
}

type LevelSettings struct {
	BallSpeed      float64 `mapstructure:"ball_speed"      json:"ball_speed"`
	PaddleSpeed    float64 `mapstructure:"paddle_speed"    json:"paddle_speed"`
	Points         int     `mapstructure:"points"          json:"points"`
	JitterRange    float64 `mapstructure:"jitter_range"    json:"jitter_range"`
	JitterInterval float64 `mapstructure:"jitter_interval" json:"jitter_interval"`
	ErrorChance    float64 `mapstructure:"error_chance"    json:"error_chance"`
	ErrorRange     float64 `mapstructure:"error_range"     json:"error_range"`
	ErrorInterval  float64 `mapstructure:"error_interval"  json:"error_interval"`
	DeadZone       float64 `mapstructure:"dead_zone"       json:"dead_zone"`
}

var gameConfig GameConfig

func loadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".") // katalog roboczy (obok binarki)
	viper.AddConfigPath("$HOME/.reverse-pong")

	// Defaults — gra działa nawet bez pliku config.yaml
	viper.SetDefault("game.duration", 60)
	viper.SetDefault("game.ball_speedup_rate", 1.05)
	viper.SetDefault("game.player_speed", 280)
	viper.SetDefault("game.hit_freeze_duration", 0.8)
	viper.SetDefault("game.countdown_digit_duration", 0.8)
	viper.SetDefault("game.go_display_duration", 0.6)

	viper.SetDefault("levels.easy.ball_speed", 240)
	viper.SetDefault("levels.easy.paddle_speed", 180)
	viper.SetDefault("levels.easy.points", 1)
	viper.SetDefault("levels.easy.jitter_range", 30)
	viper.SetDefault("levels.easy.jitter_interval", 0.5)
	viper.SetDefault("levels.easy.dead_zone", 15)

	viper.SetDefault("levels.medium.ball_speed", 300)
	viper.SetDefault("levels.medium.paddle_speed", 300)
	viper.SetDefault("levels.medium.points", 2)
	viper.SetDefault("levels.medium.error_chance", 0.30)
	viper.SetDefault("levels.medium.error_range", 70)
	viper.SetDefault("levels.medium.error_interval", 1.8)
	viper.SetDefault("levels.medium.dead_zone", 12)

	viper.SetDefault("levels.hard.ball_speed", 360)
	viper.SetDefault("levels.hard.paddle_speed", 420)
	viper.SetDefault("levels.hard.points", 3)
	viper.SetDefault("levels.hard.error_chance", 0.08)
	viper.SetDefault("levels.hard.error_range", 35)
	viper.SetDefault("levels.hard.error_interval", 3.0)
	viper.SetDefault("levels.hard.dead_zone", 3)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("config.yaml not found, using defaults")
		} else {
			log.Fatalf("config read error: %v", err)
		}
	} else {
		log.Printf("config loaded from: %s", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&gameConfig); err != nil {
		log.Fatalf("config unmarshal error: %v", err)
	}
}

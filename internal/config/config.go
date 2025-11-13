package config

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	Mode       string        `mapstructure:"mode"`
	Port       int           `mapstructure:"port"`
	StaticPath string        `mapstructure:"static_path"`
	ReadLimit  int64         `mapstructure:"read_limit"`
	PingPeriod time.Duration `mapstructure:"ping_period"`
	Secret     string        `mapstructure:"secret"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	env := os.Getenv("CONFIG_ENV")
	if env == "" {
		env = "dev"
	}
	fileName := fmt.Sprintf("config/config.%s.yaml", env)

	v.SetConfigFile(fileName)
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	v.SetDefault("mode", "release")
	v.SetDefault("port", 8080)
	v.SetDefault("static_path", "./web")
	v.SetDefault("read_limit", 32768)
	v.SetDefault("ping_period", "54s")

	if err := v.ReadInConfig(); err != nil {
		log.Warn().Str("file", fileName).Msg("Config file not found, using defaults")
	} else {
		log.Info().Str("file", fileName).Msg("Loaded config")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	log.Info().Str("mode", cfg.Mode).Int("port", cfg.Port).Str("static_path", cfg.StaticPath).Msg("Configuration summary")
	return &cfg, nil
}

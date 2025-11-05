package config

import (
	"fmt"
	"os"
	"time"

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
		fmt.Printf("⚠️ Config file not found (%s), using defaults\n", fileName)
	} else {
		fmt.Printf("✅ Loaded config: %s\n", fileName)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	fmt.Printf("🧩 Mode: %s | Port: %d | Static: %s\n", cfg.Mode, cfg.Port, cfg.StaticPath)
	return &cfg, nil
}

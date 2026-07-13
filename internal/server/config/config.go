// Package config загружает конфигурацию сервера из env и флагов.
package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config — параметры запуска сервера GophKeeper.
type Config struct {
	RunAddress  string `env:"RUN_ADDRESS"`
	DatabaseURI string `env:"DATABASE_URI"`
	JWTSecret   string `env:"JWT_SECRET"`
}

// NewConfig читает окружение, затем флаги (флаги имеют приоритет).
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	runAddress := flag.String("a", "", "address and port to listen on")
	databaseURI := flag.String("d", "", "PostgreSQL connection URI")
	jwtSecret := flag.String("j", "", "JWT signing secret")
	flag.Parse()

	if *runAddress != "" {
		cfg.RunAddress = *runAddress
	}
	if *databaseURI != "" {
		cfg.DatabaseURI = *databaseURI
	}
	if *jwtSecret != "" {
		cfg.JWTSecret = *jwtSecret
	}

	if cfg.RunAddress == "" {
		cfg.RunAddress = "localhost:8080"
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-insecure-secret-change-me"
	}

	return cfg, nil
}

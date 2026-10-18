// Package config загружает конфигурацию сервера из env и флагов.
package config

import (
	"flag"
	"fmt"
	"io"

	"github.com/caarlos0/env/v11"
)

// Config — параметры запуска сервера GophKeeper.
type Config struct {
	RunAddress  string `env:"RUN_ADDRESS"`
	DatabaseURI string `env:"DATABASE_URI"`
	JWTSecret   string `env:"JWT_SECRET"`
}

// NewConfig читает окружение, затем флаги из args (флаги имеют приоритет).
// Использует локальный FlagSet, не затрагивая глобальный flag.CommandLine.
func NewConfig(args []string) (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	fs := flag.NewFlagSet("gophkeeper-server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	runAddress := fs.String("a", "", "address and port to listen on")
	databaseURI := fs.String("d", "", "PostgreSQL connection URI")
	jwtSecret := fs.String("j", "", "JWT signing secret")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

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

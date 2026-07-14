package config_test

import (
	"testing"

	"gophkeeper/internal/server/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigDefaults(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "")
	t.Setenv("DATABASE_URI", "")
	t.Setenv("JWT_SECRET", "")

	cfg, err := config.NewConfig(nil)
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", cfg.RunAddress)
	assert.Empty(t, cfg.DatabaseURI)
	assert.NotEmpty(t, cfg.JWTSecret)
}

func TestNewConfigFlagsOverride(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "env-addr")
	t.Setenv("DATABASE_URI", "postgres://env/db")
	t.Setenv("JWT_SECRET", "env-secret")

	cfg, err := config.NewConfig([]string{
		"-a", "127.0.0.1:9090",
		"-d", "postgres://localhost/gophkeeper",
		"-j", "secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9090", cfg.RunAddress)
	assert.Equal(t, "postgres://localhost/gophkeeper", cfg.DatabaseURI)
	assert.Equal(t, "secret", cfg.JWTSecret)
}

func TestNewConfigEnvOnly(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "env:8081")
	t.Setenv("DATABASE_URI", "postgres://env/gophkeeper")
	t.Setenv("JWT_SECRET", "env-secret")

	cfg, err := config.NewConfig(nil)
	require.NoError(t, err)
	assert.Equal(t, "env:8081", cfg.RunAddress)
	assert.Equal(t, "postgres://env/gophkeeper", cfg.DatabaseURI)
	assert.Equal(t, "env-secret", cfg.JWTSecret)
}

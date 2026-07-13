package config_test

import (
	"flag"
	"os"
	"testing"

	"gophkeeper/internal/server/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigDefaults(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"server"}
	t.Setenv("RUN_ADDRESS", "")
	t.Setenv("DATABASE_URI", "")
	t.Setenv("JWT_SECRET", "")

	cfg, err := config.NewConfig()
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", cfg.RunAddress)
	assert.Empty(t, cfg.DatabaseURI)
	assert.NotEmpty(t, cfg.JWTSecret)
}

func TestNewConfigFlagsOverride(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"server", "-a", "127.0.0.1:9090", "-d", "postgres://localhost/gophkeeper", "-j", "secret"}
	t.Setenv("RUN_ADDRESS", "env-addr")
	t.Setenv("DATABASE_URI", "postgres://env/db")
	t.Setenv("JWT_SECRET", "env-secret")

	cfg, err := config.NewConfig()
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9090", cfg.RunAddress)
	assert.Equal(t, "postgres://localhost/gophkeeper", cfg.DatabaseURI)
	assert.Equal(t, "secret", cfg.JWTSecret)
}

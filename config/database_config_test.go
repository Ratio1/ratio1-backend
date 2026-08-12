package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDatabaseConfig_LoadsOnlyDatabaseSettings(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.devnet.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"Database": {
			"MaxOpenConns": 100,
			"MaxIdleConns": 50,
			"SslMode": "require"
		}
	}`), 0o600))

	t.Setenv("DATABASE_NAME", "ratio1")
	t.Setenv("DATABASE_USER", "backend")
	t.Setenv("DATABASE_HOST", "database.internal")
	t.Setenv("DATABASE_PORT", "26257")
	t.Setenv("DATABASE_PASSWORD", "secret")

	database, err := LoadDatabaseConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, &DatabaseConfig{
		DbName:       "ratio1",
		User:         "backend",
		Host:         "database.internal",
		Port:         26257,
		Password:     "secret",
		MaxOpenConns: 100,
		MaxIdleConns: 50,
		SslMode:      "require",
	}, database)
}

func TestLoadDatabaseConfig_RequiresDatabaseEnvironment(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.devnet.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"Database": {
			"MaxOpenConns": 100,
			"MaxIdleConns": 50,
			"SslMode": "require"
		}
	}`), 0o600))

	t.Setenv("DATABASE_NAME", "")
	t.Setenv("DATABASE_USER", "")
	t.Setenv("DATABASE_HOST", "")
	t.Setenv("DATABASE_PORT", "")
	t.Setenv("DATABASE_PASSWORD", "")

	_, err := LoadDatabaseConfig(configPath)
	require.EqualError(t, err, "DATABASE_NAME is not set")
}

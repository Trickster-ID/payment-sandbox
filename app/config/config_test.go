package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_NormalizeAppEnv(t *testing.T) {
	keys := []string{
		"APP_ENV",
		"JWT_SECRET",
		"JWT_DURATION_MINUTES",
		"APP_PORT",
		"SHUTDOWN_TIMEOUT_SECONDS",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSLMODE",
		"MONGO_URI",
		"MONGO_DB_NAME",
		"MONGO_COLLECTION",
		"MONGO_JOURNEY_ENABLE",
	}

	tests := []struct {
		name      string
		appEnv    string
		expected  string
		secret    string
		duration  string
		wantError bool
	}{
		{
			name:      "local allows default secret fallback",
			appEnv:    "local",
			expected:  AppEnvLocal,
			secret:    "",
			duration:  "",
			wantError: false,
		},
		{
			name:      "unknown app env is normalized to local",
			appEnv:    "sandbox-x",
			expected:  AppEnvLocal,
			secret:    "",
			duration:  "",
			wantError: false,
		},
		{
			name:      "prod rejects default secret",
			appEnv:    "prod",
			expected:  AppEnvProd,
			secret:    "change-me-in-env",
			duration:  "60",
			wantError: true,
		},
		{
			name:      "staging rejects empty secret",
			appEnv:    "staging",
			expected:  AppEnvStaging,
			secret:    "   ",
			duration:  "60",
			wantError: true,
		},
		{
			name:      "dev accepts custom secret and positive duration",
			appEnv:    "dev",
			expected:  AppEnvDev,
			secret:    "dev-super-secret-123",
			duration:  "90",
			wantError: false,
		},
		{
			name:      "prod duration fallback remains valid",
			appEnv:    "prod",
			expected:  AppEnvProd,
			secret:    "prod-super-secret-xyz",
			duration:  "0",
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "")
			}

			t.Setenv("APP_ENV", tc.appEnv)
			if tc.secret != "" {
				t.Setenv("JWT_SECRET", tc.secret)
			}
			if tc.duration != "" {
				t.Setenv("JWT_DURATION_MINUTES", tc.duration)
			}

			cfg := Load()
			assert.Equal(t, tc.expected, cfg.AppEnv)

			err := cfg.Validate()
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantError bool
	}{
		{
			name: "prod invalid non-positive duration",
			cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "prod-secret",
				JWTDuration: 0,
			},
			wantError: true,
		},
		{
			name: "prod valid secret and duration",
			cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "prod-secret",
				JWTDuration: 60,
			},
			wantError: false,
		},
		{
			name: "local allows default secret",
			cfg: Config{
				AppEnv:      AppEnvLocal,
				JWTSecret:   "change-me-in-env",
				JWTDuration: 0,
			},
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

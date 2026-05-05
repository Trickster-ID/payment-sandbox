package config

// Branch analysis
//
// normalizeAppEnv(raw):
// ├── toLower+trim == "dev"     → AppEnvDev
// ├── toLower+trim == "staging" → AppEnvStaging
// ├── toLower+trim == "prod"    → AppEnvProd
// └── anything else (incl. "local", "", unknown) → AppEnvLocal
//
// getEnv(key, fallback):
// ├── os.Getenv(key) != "" → return env value
// └── os.Getenv(key) == "" → return fallback
//
// getEnvDuration(key, fallbackMinutes):
// ├── raw == ""          → fallback duration
// ├── Atoi error         → fallback duration
// ├── minutes <= 0       → fallback duration
// └── minutes > 0        → Duration(minutes) * Minute
//
// getEnvInt(key, fallback):
// ├── raw == ""          → fallback
// ├── Atoi error         → fallback
// ├── value <= 0         → fallback
// └── value > 0          → parsed value
//
// getEnvBool(key, fallback):
// ├── raw == ""          → fallback
// ├── ParseBool error    → fallback
// └── parsed bool        → value
//
// Validate():
// ├── AppEnv == "local"                           → nil
// ├── secret (trimmed) == ""                      → error: "JWT_SECRET must be set"
// ├── secret == "change-me-in-env"                → error: "JWT_SECRET uses insecure default value"
// ├── secret == "supersecretkey"                  → error: "JWT_SECRET uses insecure default value"
// ├── JWTDuration <= 0                            → error: "JWT_DURATION_MINUTES must be greater than zero"
// └── valid secret + positive duration            → nil
//
// Load():
// └── delegates to helpers; key behaviour is default values and env-var overrides

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// not parallel (global): all tests in this file mutate process env vars via t.Setenv.

// ─── normalizeAppEnv ─────────────────────────────────────────────────────────

func TestNormalizeAppEnv(t *testing.T) {
	type args struct {
		raw string
	}
	type wants struct {
		result string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. \"dev\" -> AppEnvDev",
			args:  args{raw: "dev"},
			wants: wants{result: AppEnvDev},
		},
		{
			name:  "2. \"DEV\" uppercase -> AppEnvDev (case-insensitive)",
			args:  args{raw: "DEV"},
			wants: wants{result: AppEnvDev},
		},
		{
			name:  "3. \"  Dev  \" with whitespace -> AppEnvDev (trimmed)",
			args:  args{raw: "  Dev  "},
			wants: wants{result: AppEnvDev},
		},
		{
			name:  "4. \"staging\" -> AppEnvStaging",
			args:  args{raw: "staging"},
			wants: wants{result: AppEnvStaging},
		},
		{
			name:  "5. \"STAGING\" uppercase -> AppEnvStaging",
			args:  args{raw: "STAGING"},
			wants: wants{result: AppEnvStaging},
		},
		{
			name:  "6. \"prod\" -> AppEnvProd",
			args:  args{raw: "prod"},
			wants: wants{result: AppEnvProd},
		},
		{
			name:  "7. \"PROD\" uppercase -> AppEnvProd",
			args:  args{raw: "PROD"},
			wants: wants{result: AppEnvProd},
		},
		{
			name:  "8. \"local\" -> AppEnvLocal (default branch)",
			args:  args{raw: "local"},
			wants: wants{result: AppEnvLocal},
		},
		{
			name:  "9. \"\" empty string -> AppEnvLocal (default branch)",
			args:  args{raw: ""},
			wants: wants{result: AppEnvLocal},
		},
		{
			name:  "10. \"sandbox-x\" unknown value -> AppEnvLocal (default branch)",
			args:  args{raw: "sandbox-x"},
			wants: wants{result: AppEnvLocal},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: no shared state for this helper, but kept serial
			// for consistency with the rest of the file which uses t.Setenv
			got := normalizeAppEnv(tt.args.raw)
			assert.Equal(t, tt.wants.result, got, "normalizeAppEnv result")
		})
	}
}

// ─── getEnv ──────────────────────────────────────────────────────────────────

func TestGetEnv(t *testing.T) {
	type args struct {
		key      string
		envValue string // empty means "do not set the var"
		fallback string
	}
	type wants struct {
		result string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. env var set to non-empty value -> returns env value",
			args:  args{key: "TEST_GET_ENV_KEY", envValue: "my-value", fallback: "fallback"},
			wants: wants{result: "my-value"},
		},
		{
			name:  "2. env var not set -> returns fallback",
			args:  args{key: "TEST_GET_ENV_UNSET", envValue: "", fallback: "default-val"},
			wants: wants{result: "default-val"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: t.Setenv manipulates global env
			if tt.args.envValue != "" {
				t.Setenv(tt.args.key, tt.args.envValue)
			}

			got := getEnv(tt.args.key, tt.args.fallback)
			assert.Equal(t, tt.wants.result, got, "getEnv result")
		})
	}
}

// ─── getEnvDuration ──────────────────────────────────────────────────────────

func TestGetEnvDuration(t *testing.T) {
	const key = "TEST_DURATION_KEY"

	type args struct {
		envValue       string // empty means "do not set"
		fallbackMinutes int
	}
	type wants struct {
		result time.Duration
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. env var not set -> returns fallback duration",
			args:  args{envValue: "", fallbackMinutes: 60},
			wants: wants{result: 60 * time.Minute},
		},
		{
			name:  "2. env var set to valid positive integer -> returns parsed duration",
			args:  args{envValue: "30", fallbackMinutes: 60},
			wants: wants{result: 30 * time.Minute},
		},
		{
			name:  "3. env var set to non-numeric string -> returns fallback duration",
			args:  args{envValue: "not-a-number", fallbackMinutes: 60},
			wants: wants{result: 60 * time.Minute},
		},
		{
			name:  "4. env var set to \"0\" -> returns fallback duration (<=0 branch)",
			args:  args{envValue: "0", fallbackMinutes: 60},
			wants: wants{result: 60 * time.Minute},
		},
		{
			name:  "5. env var set to \"-5\" negative -> returns fallback duration (<=0 branch)",
			args:  args{envValue: "-5", fallbackMinutes: 60},
			wants: wants{result: 60 * time.Minute},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: t.Setenv manipulates global env
			t.Setenv(key, "")
			if tt.args.envValue != "" {
				t.Setenv(key, tt.args.envValue)
			}

			got := getEnvDuration(key, tt.args.fallbackMinutes)
			assert.Equal(t, tt.wants.result, got, "getEnvDuration result")
		})
	}
}

// ─── getEnvInt ───────────────────────────────────────────────────────────────

func TestGetEnvInt(t *testing.T) {
	const key = "TEST_INT_KEY"

	type args struct {
		envValue string // empty means "do not set"
		fallback int
	}
	type wants struct {
		result int
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. env var not set -> returns fallback",
			args:  args{envValue: "", fallback: 10},
			wants: wants{result: 10},
		},
		{
			name:  "2. env var set to valid positive integer -> returns parsed value",
			args:  args{envValue: "30", fallback: 10},
			wants: wants{result: 30},
		},
		{
			name:  "3. env var set to non-numeric string -> returns fallback",
			args:  args{envValue: "abc", fallback: 10},
			wants: wants{result: 10},
		},
		{
			name:  "4. env var set to \"0\" -> returns fallback (<=0 branch)",
			args:  args{envValue: "0", fallback: 10},
			wants: wants{result: 10},
		},
		{
			name:  "5. env var set to \"-3\" negative -> returns fallback (<=0 branch)",
			args:  args{envValue: "-3", fallback: 10},
			wants: wants{result: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: t.Setenv manipulates global env
			t.Setenv(key, "")
			if tt.args.envValue != "" {
				t.Setenv(key, tt.args.envValue)
			}

			got := getEnvInt(key, tt.args.fallback)
			assert.Equal(t, tt.wants.result, got, "getEnvInt result")
		})
	}
}

// ─── getEnvBool ──────────────────────────────────────────────────────────────

func TestGetEnvBool(t *testing.T) {
	const key = "TEST_BOOL_KEY"

	type args struct {
		envValue string // empty means "do not set"
		fallback bool
	}
	type wants struct {
		result bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. env var not set, fallback true -> returns true",
			args:  args{envValue: "", fallback: true},
			wants: wants{result: true},
		},
		{
			name:  "2. env var not set, fallback false -> returns false",
			args:  args{envValue: "", fallback: false},
			wants: wants{result: false},
		},
		{
			name:  "3. env var set to \"true\" -> returns true",
			args:  args{envValue: "true", fallback: false},
			wants: wants{result: true},
		},
		{
			name:  "4. env var set to \"false\" -> returns false",
			args:  args{envValue: "false", fallback: true},
			wants: wants{result: false},
		},
		{
			name:  "5. env var set to \"1\" -> returns true (ParseBool accepts 1)",
			args:  args{envValue: "1", fallback: false},
			wants: wants{result: true},
		},
		{
			name:  "6. env var set to \"0\" -> returns false (ParseBool accepts 0)",
			args:  args{envValue: "0", fallback: true},
			wants: wants{result: false},
		},
		{
			name:  "7. env var set to invalid value -> returns fallback",
			args:  args{envValue: "not-a-bool", fallback: true},
			wants: wants{result: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: t.Setenv manipulates global env
			t.Setenv(key, "")
			if tt.args.envValue != "" {
				t.Setenv(key, tt.args.envValue)
			}

			got := getEnvBool(key, tt.args.fallback)
			assert.Equal(t, tt.wants.result, got, "getEnvBool result")
		})
	}
}

// ─── Config.Validate ─────────────────────────────────────────────────────────

func TestConfig_Validate(t *testing.T) {
	type args struct {
		cfg Config
	}
	type wants struct {
		errMsg string // empty means no error expected
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. AppEnv=local -> nil (local bypasses all validation)",
			args: args{cfg: Config{
				AppEnv:      AppEnvLocal,
				JWTSecret:   "",
				JWTDuration: 0,
			}},
			wants: wants{errMsg: ""},
		},
		{
			name: "2. AppEnv=dev, valid secret and positive duration -> nil",
			args: args{cfg: Config{
				AppEnv:      AppEnvDev,
				JWTSecret:   "dev-strong-secret-xyz",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: ""},
		},
		{
			name: "3. AppEnv=staging, valid secret and positive duration -> nil",
			args: args{cfg: Config{
				AppEnv:      AppEnvStaging,
				JWTSecret:   "staging-strong-secret-xyz",
				JWTDuration: 30 * time.Minute,
			}},
			wants: wants{errMsg: ""},
		},
		{
			name: "4. AppEnv=prod, valid secret and positive duration -> nil",
			args: args{cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "prod-strong-secret-xyz",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: ""},
		},
		{
			name: "5. AppEnv=dev, empty secret -> error: JWT_SECRET must be set",
			args: args{cfg: Config{
				AppEnv:      AppEnvDev,
				JWTSecret:   "",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_SECRET must be set for APP_ENV=%s", AppEnvDev)},
		},
		{
			name: "6. AppEnv=prod, whitespace-only secret -> error: JWT_SECRET must be set",
			args: args{cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "   ",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_SECRET must be set for APP_ENV=%s", AppEnvProd)},
		},
		{
			name: "7. AppEnv=staging, secret=\"change-me-in-env\" -> error: insecure default",
			args: args{cfg: Config{
				AppEnv:      AppEnvStaging,
				JWTSecret:   "change-me-in-env",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_SECRET uses insecure default value for APP_ENV=%s", AppEnvStaging)},
		},
		{
			name: "8. AppEnv=prod, secret=\"supersecretkey\" -> error: insecure default",
			args: args{cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "supersecretkey",
				JWTDuration: 60 * time.Minute,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_SECRET uses insecure default value for APP_ENV=%s", AppEnvProd)},
		},
		{
			name: "9. AppEnv=prod, valid secret, JWTDuration=0 -> error: duration must be > 0",
			args: args{cfg: Config{
				AppEnv:      AppEnvProd,
				JWTSecret:   "prod-strong-secret-xyz",
				JWTDuration: 0,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_DURATION_MINUTES must be greater than zero for APP_ENV=%s", AppEnvProd)},
		},
		{
			name: "10. AppEnv=dev, valid secret, JWTDuration negative -> error: duration must be > 0",
			args: args{cfg: Config{
				AppEnv:      AppEnvDev,
				JWTSecret:   "dev-strong-secret-xyz",
				JWTDuration: -1 * time.Minute,
			}},
			wants: wants{errMsg: fmt.Sprintf("JWT_DURATION_MINUTES must be greater than zero for APP_ENV=%s", AppEnvDev)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: no env vars here, but kept serial for file consistency

			err := tt.args.cfg.Validate()

			if tt.wants.errMsg != "" {
				assert.EqualError(t, err, tt.wants.errMsg, "error message")
			} else {
				assert.NoError(t, err, "unexpected error")
			}
		})
	}
}

// ─── Load ────────────────────────────────────────────────────────────────────

func TestLoad(t *testing.T) {
	// all env vars that Load reads — cleared before each case so defaults are predictable
	allKeys := []string{
		"APP_ENV", "APP_PORT", "JWT_SECRET", "JWT_DURATION_MINUTES",
		"SHUTDOWN_TIMEOUT_SECONDS", "DB_HOST", "DB_PORT", "DB_USER",
		"DB_PASSWORD", "DB_NAME", "DB_SSLMODE", "MONGO_URI", "MONGO_DB_NAME",
		"MONGO_JOURNEY_ENABLE", "REDIS_URL",
		"OAUTH2_ACCESS_TOKEN_DURATION_MINUTES",
		"OAUTH2_REFRESH_TOKEN_DURATION_DAYS",
		"OAUTH2_AUTH_CODE_DURATION_MINUTES",
	}

	type args struct {
		envOverrides map[string]string
	}
	type wants struct {
		appEnv                     string
		appPort                    string
		jwtSecret                  string
		jwtDuration                time.Duration
		shutdownTTL                time.Duration
		dbHost                     string
		dbPort                     string
		mongoJourneyEnable         bool
		oauth2AccessTokenDuration  time.Duration
		oauth2RefreshTokenDuration time.Duration
		oauth2AuthCodeDuration     time.Duration
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. no env vars set -> all fields use documented defaults",
			args: args{envOverrides: map[string]string{}},
			wants: wants{
				appEnv:                     AppEnvLocal,
				appPort:                    "8080",
				jwtSecret:                  "change-me-in-env",
				jwtDuration:                60 * time.Minute,
				shutdownTTL:                10 * time.Second,
				dbHost:                     "127.0.0.1",
				dbPort:                     "5432",
				mongoJourneyEnable:         true,
				oauth2AccessTokenDuration:  15 * time.Minute,
				oauth2RefreshTokenDuration: 30 * 24 * time.Hour,
				oauth2AuthCodeDuration:     10 * time.Minute,
			},
		},
		{
			name: "2. env vars set to custom values -> fields reflect overrides",
			args: args{envOverrides: map[string]string{
				"APP_ENV":                              "prod",
				"APP_PORT":                             "9090",
				"JWT_SECRET":                           "my-custom-secret",
				"JWT_DURATION_MINUTES":                 "120",
				"SHUTDOWN_TIMEOUT_SECONDS":             "30",
				"DB_HOST":                              "db.example.com",
				"DB_PORT":                              "5433",
				"MONGO_JOURNEY_ENABLE":                 "false",
				"OAUTH2_ACCESS_TOKEN_DURATION_MINUTES": "5",
				"OAUTH2_REFRESH_TOKEN_DURATION_DAYS":   "7",
				"OAUTH2_AUTH_CODE_DURATION_MINUTES":    "2",
			}},
			wants: wants{
				appEnv:                     AppEnvProd,
				appPort:                    "9090",
				jwtSecret:                  "my-custom-secret",
				jwtDuration:                120 * time.Minute,
				shutdownTTL:                30 * time.Second,
				dbHost:                     "db.example.com",
				dbPort:                     "5433",
				mongoJourneyEnable:         false,
				oauth2AccessTokenDuration:  5 * time.Minute,
				oauth2RefreshTokenDuration: 7 * 24 * time.Hour,
				oauth2AuthCodeDuration:     2 * time.Minute,
			},
		},
		{
			name: "3. APP_ENV set to unknown value -> normalised to local",
			args: args{envOverrides: map[string]string{
				"APP_ENV": "unknown-env",
			}},
			wants: wants{
				appEnv:                     AppEnvLocal,
				appPort:                    "8080",
				jwtSecret:                  "change-me-in-env",
				jwtDuration:                60 * time.Minute,
				shutdownTTL:                10 * time.Second,
				dbHost:                     "127.0.0.1",
				dbPort:                     "5432",
				mongoJourneyEnable:         true,
				oauth2AccessTokenDuration:  15 * time.Minute,
				oauth2RefreshTokenDuration: 30 * 24 * time.Hour,
				oauth2AuthCodeDuration:     10 * time.Minute,
			},
		},
		{
			name: "4. invalid numeric env vars -> all fall back to defaults",
			args: args{envOverrides: map[string]string{
				"JWT_DURATION_MINUTES":                 "not-a-number",
				"SHUTDOWN_TIMEOUT_SECONDS":             "bad",
				"OAUTH2_ACCESS_TOKEN_DURATION_MINUTES": "bad",
				"OAUTH2_REFRESH_TOKEN_DURATION_DAYS":   "bad",
				"OAUTH2_AUTH_CODE_DURATION_MINUTES":    "bad",
			}},
			wants: wants{
				appEnv:                     AppEnvLocal,
				appPort:                    "8080",
				jwtSecret:                  "change-me-in-env",
				jwtDuration:                60 * time.Minute,
				shutdownTTL:                10 * time.Second,
				dbHost:                     "127.0.0.1",
				dbPort:                     "5432",
				mongoJourneyEnable:         true,
				oauth2AccessTokenDuration:  15 * time.Minute,
				oauth2RefreshTokenDuration: 30 * 24 * time.Hour,
				oauth2AuthCodeDuration:     10 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: t.Setenv manipulates global env
			for _, key := range allKeys {
				t.Setenv(key, "")
			}
			for k, v := range tt.args.envOverrides {
				t.Setenv(k, v)
			}

			got := Load()

			assert.Equal(t, tt.wants.appEnv, got.AppEnv, "AppEnv")
			assert.Equal(t, tt.wants.appPort, got.AppPort, "AppPort")
			assert.Equal(t, tt.wants.jwtSecret, got.JWTSecret, "JWTSecret")
			assert.Equal(t, tt.wants.jwtDuration, got.JWTDuration, "JWTDuration")
			assert.Equal(t, tt.wants.shutdownTTL, got.ShutdownTTL, "ShutdownTTL")
			assert.Equal(t, tt.wants.dbHost, got.DBHost, "DBHost")
			assert.Equal(t, tt.wants.dbPort, got.DBPort, "DBPort")
			assert.Equal(t, tt.wants.mongoJourneyEnable, got.MongoJourneyEnable, "MongoJourneyEnable")
			assert.Equal(t, tt.wants.oauth2AccessTokenDuration, got.OAuth2AccessTokenDuration, "OAuth2AccessTokenDuration")
			assert.Equal(t, tt.wants.oauth2RefreshTokenDuration, got.OAuth2RefreshTokenDuration, "OAuth2RefreshTokenDuration")
			assert.Equal(t, tt.wants.oauth2AuthCodeDuration, got.OAuth2AuthCodeDuration, "OAuth2AuthCodeDuration")
		})
	}
}

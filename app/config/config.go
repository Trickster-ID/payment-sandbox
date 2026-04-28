package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv             string
	AppPort            string
	JWTSecret          string
	JWTDuration        time.Duration
	ShutdownTTL        time.Duration
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	MongoURI           string
	MongoDBName        string
	MongoCollection    string
	MongoJourneyEnable bool
}

const (
	AppEnvLocal   = "local"
	AppEnvDev     = "dev"
	AppEnvStaging = "staging"
	AppEnvProd    = "prod"
)

func Load() Config {
	_ = godotenv.Load()

	return Config{
		AppEnv:             normalizeAppEnv(getEnv("APP_ENV", AppEnvLocal)),
		AppPort:            getEnv("APP_PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-env"),
		JWTDuration:        getEnvDuration("JWT_DURATION_MINUTES", 60),
		ShutdownTTL:        time.Duration(getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 10)) * time.Second,
		DBHost:             getEnv("DB_HOST", "127.0.0.1"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "payment_sandbox"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		MongoURI:           getEnv("MONGO_URI", "mongodb://mongo_user:mongo_password@127.0.0.1:27017/?authSource=admin"),
		MongoDBName:        getEnv("MONGO_DB_NAME", "payment_sandbox"),
		MongoCollection:    getEnv("MONGO_COLLECTION", "journey_logs"),
		MongoJourneyEnable: getEnvBool("MONGO_JOURNEY_ENABLE", true),
	}
}

func (c Config) Validate() error {
	if c.AppEnv == AppEnvLocal {
		return nil
	}

	secret := strings.TrimSpace(c.JWTSecret)
	if secret == "" {
		return fmt.Errorf("JWT_SECRET must be set for APP_ENV=%s", c.AppEnv)
	}
	if secret == "change-me-in-env" || secret == "supersecretkey" {
		return fmt.Errorf("JWT_SECRET uses insecure default value for APP_ENV=%s", c.AppEnv)
	}
	if c.JWTDuration <= 0 {
		return fmt.Errorf("JWT_DURATION_MINUTES must be greater than zero for APP_ENV=%s", c.AppEnv)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallbackMinutes int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallbackMinutes) * time.Minute
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes <= 0 {
		return time.Duration(fallbackMinutes) * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func normalizeAppEnv(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AppEnvDev:
		return AppEnvDev
	case AppEnvStaging:
		return AppEnvStaging
	case AppEnvProd:
		return AppEnvProd
	default:
		return AppEnvLocal
	}
}

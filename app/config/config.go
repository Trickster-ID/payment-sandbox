package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
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

func Load() Config {
	_ = godotenv.Load()

	return Config{
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

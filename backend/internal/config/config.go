package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort  string
	AppEnv   string
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string // DB_SSLMODE: "disable" for dev, "require" or "verify-full" for production
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

type JWTConfig struct {
	Secret            string
	ExpireHours       int // duration as an int avoids repeated strconv.Atoi at every call site
	RefreshExpireDays int
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		AppPort: getEnv("APP_PORT", "8000"),
		AppEnv:  getEnv("APP_ENV", "development"),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret:            getEnv("JWT_SECRET", ""),
			ExpireHours:       getEnvInt("JWT_EXPIRE_HOURS", 24),
			RefreshExpireDays: getEnvInt("JWT_REFRESH_EXPIRE_DAYS", 7),
		},
	}

	// Fail fast: validate required config at startup (12-Factor App §III).
	// An app that silently starts with a broken config causes hard-to-diagnose
	// runtime failures — better to crash immediately with a clear message.
	required := []struct{ key, val string }{
		{"JWT_SECRET", cfg.JWT.Secret},
		{"DB_USER", cfg.Database.User},
		{"DB_PASSWORD", cfg.Database.Password},
		{"DB_NAME", cfg.Database.Name},
	}
	for _, r := range required {
		if r.val == "" {
			log.Fatalf("required environment variable %s is not set", r.key)
		}
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt reads an env var as a positive integer.
// Falls back to defaultValue when the variable is unset.
// Calls log.Fatal when the variable is set but is not a valid positive integer —
// the app cannot start safely with an invalid duration value.
func getEnvInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		log.Fatalf("environment variable %s must be a positive integer, got %q", key, s) //nolint:gosec // G706: key is a hardcoded constant; s is operator-supplied env var, not untrusted user input
	}
	return v
}

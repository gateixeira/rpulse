package config

import (
	"context"
	"fmt"
	"os"
)

type Vars struct {
	WebhookSecret string
	Port          string
	DbHost        string
	DbPort        string
	DbUser        string
	DbPassword    string
	DbName        string
	LogLevel      string
}

type Config struct {
	Vars             Vars
	SimulationCancel context.CancelFunc
}

// NewAppState creates and initializes a new application state
func NewConfig() *Config {
	vars := Vars{
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		Port:          getEnvOrDefault("PORT", "8080"),
		DbHost:        getEnvOrDefault("DB_HOST", "localhost"),
		DbPort:        getEnvOrDefault("DB_PORT", "5432"),
		DbUser:        getEnvOrDefault("DB_USER", "postgres"),
		DbPassword:    os.Getenv("DB_PASSWORD"),
		DbName:        getEnvOrDefault("DB_NAME", "rpulse"),
		LogLevel:      getEnvOrDefault("LOG_LEVEL", "info"),
	}

	return &Config{Vars: vars}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Vars.DbHost,
		c.Vars.DbPort,
		c.Vars.DbUser,
		c.Vars.DbPassword,
		c.Vars.DbName,
	)
}

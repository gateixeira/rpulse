package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	// Clear environment variables before test
	os.Clearenv()

	t.Run("with default values", func(t *testing.T) {
		config := NewConfig()

		if config.Vars.Port != "8080" {
			t.Errorf("Expected Port to be 8080, got %s", config.Vars.Port)
		}
		if config.Vars.DbHost != "localhost" {
			t.Errorf("Expected DbHost to be localhost, got %s", config.Vars.DbHost)
		}
		if config.Vars.DbPort != "5432" {
			t.Errorf("Expected DbPort to be 5432, got %s", config.Vars.DbPort)
		}
		if config.Vars.DbUser != "postgres" {
			t.Errorf("Expected DbUser to be postgres, got %s", config.Vars.DbUser)
		}
		if config.Vars.DbName != "rpulse" {
			t.Errorf("Expected DbName to be rpulse, got %s", config.Vars.DbName)
		}
		if config.Vars.LogLevel != "info" {
			t.Errorf("Expected LogLevel to be info, got %s", config.Vars.LogLevel)
		}
	})

	t.Run("with custom environment values", func(t *testing.T) {
		// Set custom environment variables
		os.Setenv("WEBHOOK_SECRET", "test-secret")
		os.Setenv("PORT", "3000")
		os.Setenv("DB_HOST", "test-host")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "test-user")
		os.Setenv("DB_PASSWORD", "test-password")
		os.Setenv("DB_NAME", "test-db")
		os.Setenv("LOG_LEVEL", "debug")

		config := NewConfig()

		if config.Vars.WebhookSecret != "test-secret" {
			t.Errorf("Expected WebhookSecret to be test-secret, got %s", config.Vars.WebhookSecret)
		}
		if config.Vars.Port != "3000" {
			t.Errorf("Expected Port to be 3000, got %s", config.Vars.Port)
		}
		if config.Vars.DbHost != "test-host" {
			t.Errorf("Expected DbHost to be test-host, got %s", config.Vars.DbHost)
		}
		if config.Vars.DbPort != "5433" {
			t.Errorf("Expected DbPort to be 5433, got %s", config.Vars.DbPort)
		}
		if config.Vars.DbUser != "test-user" {
			t.Errorf("Expected DbUser to be test-user, got %s", config.Vars.DbUser)
		}
		if config.Vars.DbPassword != "test-password" {
			t.Errorf("Expected DbPassword to be test-password, got %s", config.Vars.DbPassword)
		}
		if config.Vars.DbName != "test-db" {
			t.Errorf("Expected DbName to be test-db, got %s", config.Vars.DbName)
		}
		if config.Vars.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be debug, got %s", config.Vars.LogLevel)
		}
	})
}

func TestGetDSN(t *testing.T) {
	os.Clearenv()

	t.Run("with default values", func(t *testing.T) {
		config := NewConfig()
		expected := "host=localhost port=5432 user=postgres password= dbname=rpulse sslmode=disable"
		if dsn := config.GetDSN(); dsn != expected {
			t.Errorf("Expected DSN %s, got %s", expected, dsn)
		}
	})

	t.Run("with custom values", func(t *testing.T) {
		os.Setenv("DB_HOST", "test-host")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "test-user")
		os.Setenv("DB_PASSWORD", "test-password")
		os.Setenv("DB_NAME", "test-db")

		config := NewConfig()
		expected := "host=test-host port=5433 user=test-user password=test-password dbname=test-db sslmode=disable"
		if dsn := config.GetDSN(); dsn != expected {
			t.Errorf("Expected DSN %s, got %s", expected, dsn)
		}
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	os.Clearenv()

	tests := []struct {
		name         string
		key          string
		defaultVal   string
		envVal       string
		expected     string
		shouldSetEnv bool
	}{
		{
			name:         "returns default when env not set",
			key:          "TEST_KEY",
			defaultVal:   "default",
			envVal:       "",
			expected:     "default",
			shouldSetEnv: false,
		},
		{
			name:         "returns env value when set",
			key:          "TEST_KEY",
			defaultVal:   "default",
			envVal:       "custom",
			expected:     "custom",
			shouldSetEnv: true,
		},
		{
			name:         "handles empty default",
			key:          "TEST_KEY",
			defaultVal:   "",
			envVal:       "",
			expected:     "",
			shouldSetEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault() = %v, want %v", result, tt.expected)
			}
		})
	}
}

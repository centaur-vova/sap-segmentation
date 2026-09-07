package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Очищаем env
	os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Проверяем дефолтные значения
	if cfg.DBHost != "127.0.0.1" {
		t.Errorf("Expected DBHost=127.0.0.1, got %s", cfg.DBHost)
	}

	if cfg.DBPort != "5432" {
		t.Errorf("Expected DBPort=5432, got %s", cfg.DBPort)
	}
}

func TestLoadWithEnv(t *testing.T) {
	// Устанавливаем переменные
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5433")
	defer os.Unsetenv("DB_HOST")
	defer os.Unsetenv("DB_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DBHost != "localhost" {
		t.Errorf("Expected DBHost=localhost, got %s", cfg.DBHost)
	}
}

func TestGetDBConnectionString(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBName:     "testdb",
		DBUser:     "user",
		DBPassword: "pass",
	}

	expected := "host=localhost port=5432 user=user password=pass dbname=testdb sslmode=disable"

	if got := cfg.GetDBConnectionString(); got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

func TestGetDSN(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBName:     "testdb",
		DBUser:     "user",
		DBPassword: "pass",
	}

	expected := "postgres://user:pass@localhost:5432/testdb?sslmode=disable"

	if got := cfg.GetDSN(); got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

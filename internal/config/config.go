package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config - конфигурация приложения.
type Config struct {
	// Database settings
	DBHost     string `envconfig:"DB_HOST" default:"127.0.0.1"`
	DBPort     string `envconfig:"DB_PORT" default:"5432"`
	DBName     string `envconfig:"DB_NAME" default:"mesh_group"`
	DBUser     string `envconfig:"DB_USER" default:"postgres"`
	DBPassword string `envconfig:"DB_PASSWORD" default:"postgres"`

	// Connection settings
	ConnURI          string `envconfig:"CONN_URI" default:"http://bsm.api.iql.ru/ords/bsm/segmentation/get_segmentation"`
	ConnAuthLoginPwd string `envconfig:"CONN_AUTH_LOGIN_PWD" default:"4Dfddf5:jKlljHGH"`
	ConnUserAgent    string `envconfig:"CONN_USER_AGENT" default:"spacecount-test"`
	ConnTimeout      int    `envconfig:"CONN_TIMEOUT" default:"5"`
	ConnInterval     int    `envconfig:"CONN_INTERVAL" default:"1500"`

	// Import settings
	ImportBatchSize  int `envconfig:"IMPORT_BATCH_SIZE" default:"50"`
	LogCleanupMaxAge int `envconfig:"LOG_CLEANUP_MAX_AGE" default:"7"`

	// Logger settings
	LogDir       string `envconfig:"LOG_DIR" default:"log"`
	LogFile      string `envconfig:"LOG_FILE" default:"segmentation_import.log"`
	LogToConsole bool   `envconfig:"LOG_TO_CONSOLE" default:"true"`
	LogToFile    bool   `envconfig:"LOG_TO_FILE" default:"true"`
	LogLevel     string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat    string `envconfig:"LOG_FORMAT" default:"json"` // json or text
}

// Load - загружает конфигурацию из переменных окружения.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}
	return &cfg, nil
}

// GetDBConnectionString - формирует строку подключения к PostgreSQL.
func (c *Config) GetDBConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

// GetDSN возвращает DSN для подключения к PostgreSQL.
func (c *Config) GetDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// Package storage provides database access and persistence for MTGA data.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
)

// DB wraps the database connection and provides access to repositories.
type DB struct {
	conn *sql.DB
}

// Config holds database configuration settings.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string (DSN).
	// Example: postgres://user:pass@host:5432/dbname?sslmode=disable
	// Takes precedence over individual fields if set.
	DatabaseURL string

	// Host is the PostgreSQL server host.
	Host string

	// Port is the PostgreSQL server port. Default: 5432
	Port int

	// User is the PostgreSQL username.
	User string

	// Password is the PostgreSQL password.
	Password string

	// DBName is the PostgreSQL database name.
	DBName string

	// SSLMode sets the PostgreSQL SSL mode.
	// Options: disable, require, verify-ca, verify-full
	// Default: require (for production)
	SSLMode string

	// MaxOpenConns sets the maximum number of open connections to the database.
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections in the pool.
	// Default: 5
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum amount of time a connection may be reused.
	// Default: 5 minutes
	ConnMaxLifetime time.Duration

	// AutoMigrate automatically runs pending database migrations on Open.
	// Default: false (migrations must be run manually)
	AutoMigrate bool
}

// secretsManagerPayload is the JSON structure returned by AWS Secrets Manager
// for RDS credentials.
type secretsManagerPayload struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// DefaultConfig returns a Config with sensible default values.
// It reads DATABASE_URL from the environment if set.
func DefaultConfig() *Config {
	cfg := &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		Port:            5432,
		SSLMode:         "require",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
	return cfg
}

// ConfigFromSecretsManager parses an AWS Secrets Manager JSON payload into a Config.
// The payload must contain host, port, username, password, and dbname fields.
func ConfigFromSecretsManager(secretJSON string) (*Config, error) {
	var payload secretsManagerPayload
	if err := json.Unmarshal([]byte(secretJSON), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse secrets manager payload: %w", err)
	}
	if payload.Host == "" {
		return nil, fmt.Errorf("secrets manager payload missing 'host'")
	}
	port := payload.Port
	if port == 0 {
		port = 5432
	}
	cfg := DefaultConfig()
	cfg.Host = payload.Host
	cfg.Port = port
	cfg.User = payload.Username
	cfg.Password = payload.Password
	cfg.DBName = payload.DBName
	cfg.DatabaseURL = "" // Use individual fields; buildDSN will construct DSN
	return cfg, nil
}

// buildDSN constructs a PostgreSQL DSN from individual config fields.
func (c *Config) buildDSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, sslMode,
	)
}

// Open creates a new database connection with the given configuration.
func Open(config *Config) (*DB, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	dsn := config.buildDSN()
	if dsn == "" {
		return nil, fmt.Errorf("no database connection string configured: set DATABASE_URL or provide host/user/password/dbname")
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(config.MaxOpenConns)
	conn.SetMaxIdleConns(config.MaxIdleConns)
	conn.SetConnMaxLifetime(config.ConnMaxLifetime)

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	if config.AutoMigrate {
		mgr, err := NewMigrationManager(dsn)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to create migration manager: %w", err)
		}
		if err := mgr.Up(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		if err := mgr.Close(); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to close migration manager: %w", err)
		}
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db.conn == nil {
		return nil
	}
	return db.conn.Close()
}

// Conn returns the underlying sql.DB connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Ping verifies the database connection is alive.
func (db *DB) Ping() error {
	return db.conn.Ping()
}

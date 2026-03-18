package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// DB is a global variable to hold db connection
var DB *sql.DB

var ErrNoMatch = fmt.Errorf("no matching record")

type Database struct {
	Conn *sql.DB
}

func Initialize(username, password, database string, host string, port string) (Database, error) {
	db := Database{}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, database)
	// otelsql wraps the postgres driver to emit an OpenTelemetry span for every
	// SQL query. When APM is disabled the global tracer is a noop, so this is
	// zero-overhead. In Datadog it appears as "blog-postgres" under the blog
	// service in the Service Map and APM trace view.
	// otelsql wraps the postgres driver to emit an OpenTelemetry span for every
	// SQL query. When APM is disabled the global tracer is a noop, so this is
	// zero-overhead. In Datadog it appears as "blog-postgres" under the blog
	// service in the Service Map and APM trace view.
	conn, err := otelsql.Open("postgres", dsn,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
	if err != nil {
		return db, err
	}

	if _, err := otelsql.RegisterDBStatsMetrics(conn,
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	); err != nil {
		logger.Warn().Err(err).Msg("db: failed to register otelsql stats metrics")
	}

	db.Conn = conn

	// Connection pool configuration
	db.Conn.SetMaxOpenConns(25)
	db.Conn.SetMaxIdleConns(10)
	db.Conn.SetConnMaxLifetime(5 * time.Minute)
	db.Conn.SetConnMaxIdleTime(5 * time.Minute)

	err = db.Conn.Ping()
	if err != nil {
		return db, err
	}
	logger.Info().Msg("database connection established")
	DB = db.Conn
	return db, nil
}

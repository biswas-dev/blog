package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
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
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return db, err
	}

	db.Conn = conn

	// Connection pool configuration
	db.Conn.SetMaxOpenConns(25)
	db.Conn.SetMaxIdleConns(10)
	db.Conn.SetConnMaxLifetime(5 * time.Minute)
	db.Conn.SetConnMaxIdleTime(1 * time.Minute)

	err = db.Conn.Ping()
	if err != nil {
		return db, err
	}
	logger.Info().Msg("database connection established")
	DB = db.Conn
	return db, nil
}

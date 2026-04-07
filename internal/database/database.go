package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	var err error
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Lỗi kết nối PostgreSQL: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Lỗi Ping PostgreSQL: %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS threads (
		id SERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		author_name TEXT DEFAULT '@anonymous',
		parent_id INTEGER REFERENCES threads(id),
		parent_author TEXT,
		category TEXT DEFAULT 'feed',
		signal_score INTEGER DEFAULT 0,
		is_hidden BOOLEAN DEFAULT FALSE,
		is_locked BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		published_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS votes (
		id SERIAL PRIMARY KEY,
		thread_id INTEGER REFERENCES threads(id),
		visitor_id TEXT NOT NULL,
		vote_type INTEGER NOT NULL,
		UNIQUE(thread_id, visitor_id)
	);

	CREATE TABLE IF NOT EXISTS thread_tags (
		id SERIAL PRIMARY KEY,
		thread_id INTEGER REFERENCES threads(id),
		visitor_id TEXT NOT NULL,
		suggested_category TEXT NOT NULL,
		UNIQUE(thread_id, visitor_id)
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`

	_, err = DB.Exec(schema)
	if err != nil {
		log.Fatalf("Lỗi khởi tạo schema Postgres: %v", err)
	}

	_, _ = DB.Exec("INSERT INTO system_settings (key, value) VALUES ('last_mod_checkin', TO_CHAR(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')) ON CONFLICT DO NOTHING")

	fmt.Println("Hệ thống Database (PostgreSQL) đã sẵn sàng cho GARDEN.")
}

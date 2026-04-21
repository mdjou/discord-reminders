package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{db}, nil
}

type migration struct {
	id    int
	query string
}

var migrations = []migration{
	{
		id: 1,
		query: `
		CREATE TABLE IF NOT EXISTS guild_settings (
			guild_id TEXT PRIMARY KEY,
			channel_id TEXT,
			ping_role_id TEXT,
			reminder_time TEXT DEFAULT '18:00',
			enabled BOOLEAN DEFAULT 1,
			default_main_text TEXT,
			default_embed_text TEXT,
			default_embed_image TEXT,
			default_embed_thumbnail TEXT
		);
		CREATE TABLE IF NOT EXISTS custom_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT,
			main_text TEXT,
			embed_text TEXT,
			embed_image TEXT,
			embed_thumbnail TEXT,
			skip BOOLEAN DEFAULT 0,
			position INTEGER,
			FOREIGN KEY(guild_id) REFERENCES guild_settings(guild_id)
		);
		CREATE TABLE IF NOT EXISTS sent_reminders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT,
			channel_id TEXT,
			sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			main_text TEXT,
			embed_text TEXT,
			embed_image TEXT,
			embed_thumbnail TEXT,
			is_custom BOOLEAN,
			FOREIGN KEY(guild_id) REFERENCES guild_settings(guild_id)
		);`,
	},
}

func migrate(db *sql.DB) error {
	// Create migrations table if it doesn't exist
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (id INTEGER PRIMARY KEY)`)
	if err != nil {
		return err
	}

	// Get current version
	var currentVersion int
	err = db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM migrations`).Scan(&currentVersion)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if m.id > currentVersion {
			fmt.Printf("Applying migration %d...\n", m.id)
			if _, err := db.Exec(m.query); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", m.id, err)
			}
			if _, err := db.Exec(`INSERT INTO migrations (id) VALUES (?)`, m.id); err != nil {
				return fmt.Errorf("failed to update migration version: %w", err)
			}
		}
	}

	return nil
}

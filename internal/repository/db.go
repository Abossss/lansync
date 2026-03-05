package repository

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func InitDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25) // Limit simultaneous connections
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute) // 5 minutes

	// Enable foreign key support
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return db, nil
}

func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			parent_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (parent_id) REFERENCES folders(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			size INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			checksum TEXT NOT NULL,
			folder_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (folder_id) REFERENCES folders(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			file_id TEXT NOT NULL,
			type TEXT NOT NULL,
			client_ip TEXT NOT NULL,
			bytes_transferred INTEGER DEFAULT 0,
			total_bytes INTEGER NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS peers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			port INTEGER NOT NULL,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			version TEXT,
			file_count INTEGER DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS share_links (
			id TEXT PRIMARY KEY,
			file_id TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			downloads INTEGER DEFAULT 0,
			max_downloads INTEGER DEFAULT 0,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_files_folder ON files(folder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_files_created ON files(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_file ON sessions(file_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_share_token ON share_links(token)`,
		`CREATE INDEX IF NOT EXISTS idx_share_expires ON share_links(expires_at)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to run migration: %w\nMigration: %s", err, migration)
		}
	}

	return nil
}

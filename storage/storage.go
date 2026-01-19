package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// FileMetadata holds information about uploaded files
type FileMetadata struct {
	ID            int64
	LinkID        string
	FileID        int64 // Telegram file ID
	AccessHash    int64
	FileReference []byte
	FileName      string
	FileSize      int64
	MimeType      string
	CreatedAt     time.Time
}

// Storage handles database operations
type Storage struct {
	db *sql.DB
}

// New creates a new Storage instance
func New(dbPath string) (*Storage, error) {
	// Create data directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// initSchema creates the necessary database tables
func (s *Storage) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id TEXT NOT NULL UNIQUE,
		file_id INTEGER NOT NULL,
		access_hash INTEGER NOT NULL DEFAULT 0,
		file_reference BLOB,
		file_name TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		mime_type TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_link_id ON files(link_id);
	`
	if _, err := s.db.Exec(query); err != nil {
		return err
	}

	// Migration: Add columns if they usually don't exist (ignoring errors for simplicity in this dev loop)
	// In a real app, check schema version.
	s.db.Exec("ALTER TABLE files ADD COLUMN access_hash INTEGER DEFAULT 0")
	s.db.Exec("ALTER TABLE files ADD COLUMN file_reference BLOB")

	return nil
}

// SaveFile stores file metadata and returns the assigned link ID
func (s *Storage) SaveFile(linkID string, fileID int64, accessHash int64, fileReference []byte, fileName string, fileSize int64, mimeType string) error {
	query := `INSERT INTO files (link_id, file_id, access_hash, file_reference, file_name, file_size, mime_type) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, linkID, fileID, accessHash, fileReference, fileName, fileSize, mimeType)
	return err
}

// GetFileByLink retrieves file metadata by link ID
func (s *Storage) GetFileByLink(linkID string) (*FileMetadata, error) {
	query := `SELECT id, link_id, file_id, access_hash, file_reference, file_name, file_size, mime_type, created_at FROM files WHERE link_id = ?`
	row := s.db.QueryRow(query, linkID)

	var meta FileMetadata
	err := row.Scan(&meta.ID, &meta.LinkID, &meta.FileID, &meta.AccessHash, &meta.FileReference, &meta.FileName, &meta.FileSize, &meta.MimeType, &meta.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &meta, nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

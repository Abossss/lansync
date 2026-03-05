package models

import (
	"time"
)

type File struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type"`
	Checksum  string    `json:"checksum"`
	FolderID  *string   `json:"folder_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	ParentID  *string   `json:"parent_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID            string    `json:"id"`
	FileID        string    `json:"file_id"`
	Type          string    `json:"type"` // upload, download
	ClientIP      string    `json:"client_ip"`
	BytesTransferred int64  `json:"bytes_transferred"`
	TotalBytes    int64     `json:"total_bytes"`
	Status        string    `json:"status"` // pending, active, completed, failed, cancelled
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Peer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	LastSeen  time.Time `json:"last_seen"`
	Version   string    `json:"version"`
	FileCount int       `json:"file_count"`
}

type TransferProgress struct {
	FileID      string `json:"file_id"`
	SessionID   string `json:"session_id"`
	Type        string `json:"type"`
	Transferred int64  `json:"transferred"`
	TotalBytes  int64  `json:"total_bytes"`
	Speed       int64  `json:"speed"` // bytes per second
	ETA         int64  `json:"eta"`   // seconds
	Status      string `json:"status"`
}

type UploadResult struct {
	FileID   string `json:"file_id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Error    string `json:"error,omitempty"`
}

// ShareLink 分享链接
type ShareLink struct {
	ID           string    `json:"id"`
	FileID       string    `json:"file_id"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	Downloads    int       `json:"downloads"`
	MaxDownloads int       `json:"max_downloads,omitempty"`
}

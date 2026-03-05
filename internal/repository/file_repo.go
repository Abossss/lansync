package repository

import (
	"database/sql"
	"time"

	"github.com/abossss/lansync/internal/models"
)

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(file *models.File) error {
	query := `
		INSERT INTO files (id, name, path, size, mime_type, checksum, folder_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		file.ID, file.Name, file.Path, file.Size, file.MimeType,
		file.Checksum, file.FolderID, file.CreatedAt, file.UpdatedAt,
	)
	return err
}

func (r *FileRepository) GetByID(id string) (*models.File, error) {
	query := `
		SELECT id, name, path, size, mime_type, checksum, folder_id, created_at, updated_at
		FROM files WHERE id = ?
	`
	file := &models.File{}
	err := r.db.QueryRow(query, id).Scan(
		&file.ID, &file.Name, &file.Path, &file.Size, &file.MimeType,
		&file.Checksum, &file.FolderID, &file.CreatedAt, &file.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return file, err
}

func (r *FileRepository) List(folderID *string, limit, offset int) ([]*models.File, error) {
	query := `
		SELECT id, name, path, size, mime_type, checksum, folder_id, created_at, updated_at
		FROM files
		WHERE folder_id IS ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, folderID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		err := rows.Scan(
			&file.ID, &file.Name, &file.Path, &file.Size, &file.MimeType,
			&file.Checksum, &file.FolderID, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (r *FileRepository) Search(query string, limit int) ([]*models.File, error) {
	searchQuery := `
		SELECT id, name, path, size, mime_type, checksum, folder_id, created_at, updated_at
		FROM files
		WHERE name LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := r.db.Query(searchQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		err := rows.Scan(
			&file.ID, &file.Name, &file.Path, &file.Size, &file.MimeType,
			&file.Checksum, &file.FolderID, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (r *FileRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

func (r *FileRepository) Update(id string, file *models.File) error {
	query := `
		UPDATE files
		SET name = ?, path = ?, size = ?, mime_type = ?, checksum = ?, folder_id = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		file.Name, file.Path, file.Size, file.MimeType,
		file.Checksum, file.FolderID, time.Now(), id,
	)
	return err
}

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *models.Session) error {
	query := `
		INSERT INTO sessions (id, file_id, type, client_ip, bytes_transferred, total_bytes, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		session.ID, session.FileID, session.Type, session.ClientIP,
		session.BytesTransferred, session.TotalBytes, session.Status,
		session.CreatedAt, session.UpdatedAt,
	)
	return err
}

func (r *SessionRepository) GetByID(id string) (*models.Session, error) {
	query := `
		SELECT id, file_id, type, client_ip, bytes_transferred, total_bytes, status, created_at, updated_at
		FROM sessions WHERE id = ?
	`
	session := &models.Session{}
	err := r.db.QueryRow(query, id).Scan(
		&session.ID, &session.FileID, &session.Type, &session.ClientIP,
		&session.BytesTransferred, &session.TotalBytes, &session.Status,
		&session.CreatedAt, &session.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return session, err
}

func (r *SessionRepository) ListActive() ([]*models.Session, error) {
	query := `
		SELECT id, file_id, type, client_ip, bytes_transferred, total_bytes, status, created_at, updated_at
		FROM sessions
		WHERE status IN ('pending', 'active')
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		session := &models.Session{}
		err := rows.Scan(
			&session.ID, &session.FileID, &session.Type, &session.ClientIP,
			&session.BytesTransferred, &session.TotalBytes, &session.Status,
			&session.CreatedAt, &session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (r *SessionRepository) UpdateStatus(id, status string) error {
	query := `UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), id)
	return err
}

func (r *SessionRepository) UpdateProgress(id string, transferred int64) error {
	query := `UPDATE sessions SET bytes_transferred = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, transferred, time.Now(), id)
	return err
}

func (r *SessionRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

type PeerRepository struct {
	db *sql.DB
}

func NewPeerRepository(db *sql.DB) *PeerRepository {
	return &PeerRepository{db: db}
}

func (r *PeerRepository) CreateOrUpdate(peer *models.Peer) error {
	query := `
		INSERT INTO peers (id, name, address, port, last_seen, version, file_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			address = excluded.address,
			port = excluded.port,
			last_seen = excluded.last_seen,
			version = excluded.version,
			file_count = excluded.file_count
	`
	_, err := r.db.Exec(query,
		peer.ID, peer.Name, peer.Address, peer.Port,
		peer.LastSeen, peer.Version, peer.FileCount,
	)
	return err
}

func (r *PeerRepository) List() ([]*models.Peer, error) {
	query := `
		SELECT id, name, address, port, last_seen, version, file_count
		FROM peers
		ORDER BY last_seen DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []*models.Peer
	for rows.Next() {
		peer := &models.Peer{}
		err := rows.Scan(
			&peer.ID, &peer.Name, &peer.Address, &peer.Port,
			&peer.LastSeen, &peer.Version, &peer.FileCount,
		)
		if err != nil {
			return nil, err
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func (r *PeerRepository) GetByID(id string) (*models.Peer, error) {
	query := `
		SELECT id, name, address, port, last_seen, version, file_count
		FROM peers WHERE id = ?
	`
	peer := &models.Peer{}
	err := r.db.QueryRow(query, id).Scan(
		&peer.ID, &peer.Name, &peer.Address, &peer.Port,
		&peer.LastSeen, &peer.Version, &peer.FileCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return peer, err
}

func (r *PeerRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM peers WHERE id = ?", id)
	return err
}

func (r *PeerRepository) Upsert(peer *models.Peer) error {
	query := `
		INSERT OR REPLACE INTO peers (id, name, address, port, last_seen, version, file_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		peer.ID, peer.Name, peer.Address, peer.Port,
		peer.LastSeen, peer.Version, peer.FileCount,
	)
	return err
}

func (r *PeerRepository) CleanupStale(timeout time.Duration) error {
	query := `DELETE FROM peers WHERE datetime(last_seen) < datetime('now', '-' || ? || ' seconds')`
	_, err := r.db.Exec(query, int(timeout.Seconds()))
	return err
}

// ShareLinkRepository 分享链接仓库
type ShareLinkRepository struct {
	db *sql.DB
}

func NewShareLinkRepository(db *sql.DB) *ShareLinkRepository {
	return &ShareLinkRepository{db: db}
}

func (r *ShareLinkRepository) Create(link *models.ShareLink) error {
	query := `
		INSERT INTO share_links (id, file_id, token, expires_at, created_at, downloads, max_downloads)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		link.ID, link.FileID, link.Token, link.ExpiresAt, link.CreatedAt, link.Downloads, link.MaxDownloads,
	)
	return err
}

func (r *ShareLinkRepository) GetByToken(token string) (*models.ShareLink, error) {
	query := `
		SELECT id, file_id, token, expires_at, created_at, downloads, max_downloads
		FROM share_links WHERE token = ?
	`
	link := &models.ShareLink{}
	err := r.db.QueryRow(query, token).Scan(
		&link.ID, &link.FileID, &link.Token, &link.ExpiresAt,
		&link.CreatedAt, &link.Downloads, &link.MaxDownloads,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return link, err
}

func (r *ShareLinkRepository) GetByFileID(fileID string) ([]*models.ShareLink, error) {
	query := `
		SELECT id, file_id, token, expires_at, created_at, downloads, max_downloads
		FROM share_links WHERE file_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*models.ShareLink
	for rows.Next() {
		link := &models.ShareLink{}
		err := rows.Scan(
			&link.ID, &link.FileID, &link.Token, &link.ExpiresAt,
			&link.CreatedAt, &link.Downloads, &link.MaxDownloads,
		)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, nil
}

func (r *ShareLinkRepository) IncrementDownloads(token string) error {
	_, err := r.db.Exec("UPDATE share_links SET downloads = downloads + 1 WHERE token = ?", token)
	return err
}

func (r *ShareLinkRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM share_links WHERE id = ?", id)
	return err
}

func (r *ShareLinkRepository) DeleteExpired() error {
	_, err := r.db.Exec("DELETE FROM share_links WHERE datetime(expires_at) < datetime('now')")
	return err
}

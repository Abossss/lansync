package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/models"
	"github.com/abossss/lansync/internal/repository"
	"github.com/abossss/lansync/internal/websocket"
)

type FileService struct {
	cfg         *config.Config
	fileRepo    *repository.FileRepository
	sessionRepo *repository.SessionRepository
	hub         *websocket.Hub
}

func NewFileService(cfg *config.Config, fileRepo *repository.FileRepository, sessionRepo *repository.SessionRepository, hub *websocket.Hub) *FileService {
	return &FileService{
		cfg:         cfg,
		fileRepo:    fileRepo,
		sessionRepo: sessionRepo,
		hub:         hub,
	}
}

// sanitizeFilename 清理文件名，防止路径遍历攻击
func sanitizeFilename(filename string) (string, error) {
	// 移除路径分隔符和危险字符
	filename = filepath.Base(filename)

	// 移除 Windows 驱动器字母 (如 C:)
	filename = regexp.MustCompile(`^[a-zA-Z]:`).ReplaceAllString(filename, "")

	// 移除危险字符
	dangerousChars := []string{"..", "\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerousChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// 移除控制字符
	filename = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, filename)

	// 限制文件名长度
	if len(filename) > 255 {
		ext := filepath.Ext(filename)
		name := filename[:255-len(ext)]
		filename = name + ext
	}

	if filename == "" || filename == "." {
		return "", fmt.Errorf("invalid filename")
	}

	return filename, nil
}

// validateFileSize 验证文件大小
func (s *FileService) validateFileSize(size int64) error {
	maxSize := s.cfg.Server.MaxUpload
	if maxSize <= 0 {
		maxSize = 1073741824 // 默认 1GB
	}

	if size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", size, maxSize)
	}

	// 检查存储空间
	used, _, err := s.GetStorageStats()
	if err != nil {
		return fmt.Errorf("failed to check storage: %w", err)
	}

	if used+size > s.cfg.Storage.MaxStorage {
		return fmt.Errorf("insufficient storage space")
	}

	return nil
}

func (s *FileService) UploadFile(src io.Reader, filename string, size int64, clientIP string) (*models.UploadResult, error) {
	// 安全验证：清理文件名
	safeFilename, err := sanitizeFilename(filename)
	if err != nil {
		return &models.UploadResult{Name: filename, Error: err.Error()}, err
	}

	// 安全验证：检查文件大小
	if err := s.validateFileSize(size); err != nil {
		return &models.UploadResult{Name: safeFilename, Error: err.Error()}, err
	}

	// Generate unique ID
	fileID := uuid.New().String()

	// Create temp file
	tempPath := filepath.Join(s.cfg.Storage.TempDir, fileID+".tmp")
	dst, err := os.Create(tempPath)
	if err != nil {
		return &models.UploadResult{Error: fmt.Sprintf("Failed to create temp file: %v", err)}, err
	}

	// Create hash writer
	hash := sha256.New()
	multiWriter := io.MultiWriter(dst, hash)

	// Copy file with progress tracking
	transferred, err := io.Copy(multiWriter, src)
	if err != nil {
		dst.Close()
		os.Remove(tempPath)
		return &models.UploadResult{Error: fmt.Sprintf("Upload failed: %v", err)}, err
	}

	// Sync and close file before moving
	dst.Sync()
	dst.Close()

	// Calculate checksum
	checksum := hex.EncodeToString(hash.Sum(nil))

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Move to final location
	finalPath := filepath.Join(s.cfg.Storage.UploadDir, fileID)
	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return &models.UploadResult{Error: fmt.Sprintf("Failed to save file: %v", err)}, err
	}

	// Create file record
	now := time.Now()
	file := &models.File{
		ID:        fileID,
		Name:      safeFilename,
		Path:      finalPath,
		Size:      transferred,
		MimeType:  mimeType,
		Checksum:  checksum,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save to database
	if err := s.fileRepo.Create(file); err != nil {
		os.Remove(finalPath)
		return &models.UploadResult{Error: fmt.Sprintf("Failed to save metadata: %v", err)}, err
	}

	return &models.UploadResult{
		FileID:   fileID,
		Name:     safeFilename,
		Size:     transferred,
		Checksum: checksum,
	}, nil
}

func (s *FileService) GetFile(fileID string) (*models.File, error) {
	return s.fileRepo.GetByID(fileID)
}

func (s *FileService) ListFiles(folderID *string, page, limit int) ([]*models.File, error) {
	offset := (page - 1) * limit
	return s.fileRepo.List(folderID, limit, offset)
}

func (s *FileService) SearchFiles(query string, limit int) ([]*models.File, error) {
	return s.fileRepo.Search(query, limit)
}

func (s *FileService) DeleteFile(fileID string) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}
	if file == nil {
		return fmt.Errorf("file not found")
	}

	// Delete from filesystem
	if err := os.Remove(file.Path); err != nil {
		return err
	}

	// Delete from database
	return s.fileRepo.Delete(fileID)
}

func (s *FileService) ServeFile(w http.ResponseWriter, r *http.Request, file *models.File) error {
	// 安全验证：检查文件路径是否在允许的目录内
	absPath, err := filepath.Abs(file.Path)
	if err != nil {
		return fmt.Errorf("invalid file path")
	}
	uploadDir, err := filepath.Abs(s.cfg.Storage.UploadDir)
	if err != nil {
		return fmt.Errorf("invalid upload directory")
	}
	if !strings.HasPrefix(absPath, uploadDir) {
		return fmt.Errorf("access denied: file outside upload directory")
	}

	// Open file
	f, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Get file info for size
	stat, err := f.Stat()
	if err != nil {
		return err
	}

	// 安全处理文件名用于 Content-Disposition
	safeName := strings.ReplaceAll(file.Name, "\"", "\\\"")

	// Set headers
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Handle range request for resume support
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		return s.serveRange(w, r, f, file, rangeHeader, stat.Size())
	}

	// Stream file
	http.ServeContent(w, r, file.Name, stat.ModTime(), f)
	return nil
}

func (s *FileService) serveRange(w http.ResponseWriter, r *http.Request, f *os.File, file *models.File, rangeHeader string, fileSize int64) error {
	// Parse range header: "bytes=start-end"
	ranges, err := parseRange(rangeHeader, fileSize)
	if err != nil {
		return err
	}

	if len(ranges) == 0 {
		// Invalid range, serve entire file
		stat, err := f.Stat()
		if err != nil {
			return err
		}
		http.ServeContent(w, r, file.Name, stat.ModTime(), f)
		return nil
	}

	rg := ranges[0]
	// Seek to start position
	if _, err := f.Seek(rg.Start, 0); err != nil {
		return err
	}

	// Set partial content headers
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rg.Start, rg.End, fileSize))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", rg.End-rg.Start+1))
	w.Header().Set("Content-Type", file.MimeType)
	w.WriteHeader(http.StatusPartialContent)

	// Copy limited range
	limitedReader := io.LimitReader(f, rg.End-rg.Start+1)
	_, err = io.Copy(w, limitedReader)
	return err
}

func parseRange(rangeHeader string, fileSize int64) ([]*httpRange, error) {
	// Simple range parser: "bytes=start-end"
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, nil
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, nil
	}

	var start, end int64
	var err error

	if parts[0] != "" {
		start, err = parseInt64(parts[0])
		if err != nil {
			return nil, err
		}
	}

	if parts[1] != "" {
		end, err = parseInt64(parts[1])
		if err != nil {
			return nil, err
		}
	} else {
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || end >= fileSize || start > end {
		return nil, nil
	}

	return []*httpRange{{Start: start, End: end}}, nil
}

func parseInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

type httpRange struct {
	Start int64
	End   int64
}

func (s *FileService) GetStorageStats() (used int64, fileCount int, err error) {
	entries, err := os.ReadDir(s.cfg.Storage.UploadDir)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, _ := entry.Info()
			used += info.Size()
			fileCount++
		}
	}

	return used, fileCount, nil
}

// StorageStats 存储统计信息
type StorageStats struct {
	UsedBytes   int64 `json:"used_bytes"`
	TotalBytes  int64 `json:"total_bytes"`
	UsedPercent int   `json:"used_percent"`
	FileCount   int   `json:"file_count"`
}

// GetStorageStatsDetail 获取详细存储统计
func (s *FileService) GetStorageStatsDetail() (*StorageStats, error) {
	used, fileCount, err := s.GetStorageStats()
	if err != nil {
		return nil, err
	}

	total := s.cfg.Storage.MaxStorage
	if total <= 0 {
		total = 10737418240 // 默认 10GB
	}

	usedPercent := 0
	if total > 0 {
		usedPercent = int((used * 100) / total)
	}

	return &StorageStats{
		UsedBytes:   used,
		TotalBytes:  total,
		UsedPercent: usedPercent,
		FileCount:   fileCount,
	}, nil
}

// PreviewFile 预览文件（返回文件内容用于在线查看）
func (s *FileService) PreviewFile(file *models.File) ([]byte, string, error) {
	// 安全验证：检查文件路径
	absPath, err := filepath.Abs(file.Path)
	if err != nil {
		return nil, "", fmt.Errorf("invalid file path")
	}
	uploadDir, _ := filepath.Abs(s.cfg.Storage.UploadDir)
	if !strings.HasPrefix(absPath, uploadDir) {
		return nil, "", fmt.Errorf("access denied")
	}

	// 限制预览文件大小 (最大 10MB)
	if file.Size > 10*1024*1024 {
		return nil, "", fmt.Errorf("file too large for preview (max 10MB)")
	}

	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, "", err
	}

	// 判断预览类型
	previewType := "binary"
	if strings.HasPrefix(file.MimeType, "image/") {
		previewType = "image"
	} else if strings.HasPrefix(file.MimeType, "text/") ||
		strings.HasPrefix(file.MimeType, "application/json") ||
		strings.HasPrefix(file.MimeType, "application/javascript") {
		previewType = "text"
	} else if file.MimeType == "application/pdf" {
		previewType = "pdf"
	}

	return data, previewType, nil
}

// GetFilesByIDs 根据ID列表获取多个文件
func (s *FileService) GetFilesByIDs(ids []string) ([]*models.File, error) {
	files := make([]*models.File, 0, len(ids))
	for _, id := range ids {
		file, err := s.fileRepo.GetByID(id)
		if err != nil {
			return nil, err
		}
		if file != nil {
			files = append(files, file)
		}
	}
	return files, nil
}

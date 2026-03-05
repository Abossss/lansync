package handlers

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/models"
	"github.com/abossss/lansync/internal/repository"
	"github.com/abossss/lansync/internal/services"
)

type FileHandler struct {
	fileService   *services.FileService
	cfg           *config.Config
	shareLinkRepo *repository.ShareLinkRepository
}

func NewFileHandler(fs *services.FileService, cfg *config.Config, slr *repository.ShareLinkRepository) *FileHandler {
	return &FileHandler{
		fileService:   fs,
		cfg:           cfg,
		shareLinkRepo: slr,
	}
}

func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 100MB in memory)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	var results []models.UploadResult
	clientIP := r.RemoteAddr
	maxUpload := h.cfg.Server.MaxUpload
	if maxUpload <= 0 {
		maxUpload = 1073741824 // 默认 1GB
	}

	for _, fileHeader := range files {
		// 先验证文件大小
		if fileHeader.Size > maxUpload {
			results = append(results, models.UploadResult{
				Name:  fileHeader.Filename,
				Error: "file size exceeds maximum allowed size",
			})
			continue
		}

		file, err := fileHeader.Open()
		if err != nil {
			results = append(results, models.UploadResult{
				Name:  fileHeader.Filename,
				Error: err.Error(),
			})
			continue
		}

		// 使用函数包装确保文件被关闭
		result, err := h.fileService.UploadFile(file, fileHeader.Filename, fileHeader.Size, clientIP)

		// 立即关闭文件，不要等待
		file.Close()

		if err != nil {
			results = append(results, models.UploadResult{
				Name:  fileHeader.Filename,
				Error: err.Error(),
			})
			continue
		}

		results = append(results, *result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pageNum, err := strconv.Atoi(p); err == nil {
			page = pageNum
		}
	}

	// 从配置读取默认每页数量
	limit := h.cfg.UI.ItemsPerPage
	if limit <= 0 {
		limit = 50
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if limitNum, err := strconv.Atoi(l); err == nil {
			limit = limitNum
		}
	}

	folderID := getQueryParamPtr(r, "folder_id")

	files, err := h.fileService.ListFiles(folderID, page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (h *FileHandler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// 从配置读取默认每页数量
	limit := h.cfg.UI.ItemsPerPage
	if limit <= 0 {
		limit = 50
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if limitNum, err := strconv.Atoi(l); err == nil {
			limit = limitNum
		}
	}

	files, err := h.fileService.SearchFiles(query, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	file, err := h.fileService.GetFile(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(file)
}

func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	file, err := h.fileService.GetFile(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if err := h.fileService.ServeFile(w, r, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	if err := h.fileService.DeleteFile(fileID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetStorageStats 获取存储统计
func (h *FileHandler) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.fileService.GetStorageStatsDetail()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// PreviewFile 预览文件
func (h *FileHandler) PreviewFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	file, err := h.fileService.GetFile(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	data, previewType, err := h.fileService.PreviewFile(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("X-Preview-Type", previewType)
	w.Write(data)
}

// BatchDownload 批量下载
func (h *FileHandler) BatchDownload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		IDs []string `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(request.IDs) == 0 {
		http.Error(w, "No file IDs provided", http.StatusBadRequest)
		return
	}

	files, err := h.fileService.GetFilesByIDs(request.IDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(files) == 0 {
		http.Error(w, "No files found", http.StatusNotFound)
		return
	}

	// 单文件直接下载
	if len(files) == 1 {
		h.fileService.ServeFile(w, r, files[0])
		return
	}

	// 多文件打包 ZIP
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"lansync_batch_%d.zip\"", time.Now().Unix()))

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, file := range files {
		data, _, err := h.fileService.PreviewFile(file)
		if err != nil {
			continue
		}

		// 使用原始文件名，添加序号避免冲突
		name := file.Name
		writer, err := zipWriter.Create(name)
		if err != nil {
			continue
		}

		writer.Write(data)
	}
}

// CreateShareLink 创建分享链接
func (h *FileHandler) CreateShareLink(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	var request struct {
		ExpiresHours int `json:"expires_hours"`
		MaxDownloads int `json:"max_downloads"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		request.ExpiresHours = 24 // 默认 24 小时
	}

	if request.ExpiresHours <= 0 {
		request.ExpiresHours = 24
	}

	// 检查文件是否存在
	file, err := h.fileService.GetFile(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// 生成随机 token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	link := &models.ShareLink{
		ID:           generateID(),
		FileID:       fileID,
		Token:        token,
		ExpiresAt:    time.Now().Add(time.Duration(request.ExpiresHours) * time.Hour),
		CreatedAt:    time.Now(),
		Downloads:    0,
		MaxDownloads: request.MaxDownloads,
	}

	if err := h.shareLinkRepo.Create(link); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"share_id":    link.ID,
		"token":       link.Token,
		"share_url":   fmt.Sprintf("/share/%s", link.Token),
		"expires_at":  link.ExpiresAt,
		"file_name":   file.Name,
	})
}

// DownloadByShare 通过分享链接下载
func (h *FileHandler) DownloadByShare(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	link, err := h.shareLinkRepo.GetByToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if link == nil {
		http.Error(w, "Share link not found", http.StatusNotFound)
		return
	}

	// 检查是否过期
	if time.Now().After(link.ExpiresAt) {
		http.Error(w, "Share link has expired", http.StatusGone)
		return
	}

	// 检查下载次数
	if link.MaxDownloads > 0 && link.Downloads >= link.MaxDownloads {
		http.Error(w, "Download limit reached", http.StatusForbidden)
		return
	}

	file, err := h.fileService.GetFile(link.FileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// 增加下载计数
	h.shareLinkRepo.IncrementDownloads(token)

	// 提供文件下载
	h.fileService.ServeFile(w, r, file)
}

// GetShareInfo 获取分享链接信息
func (h *FileHandler) GetShareInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	link, err := h.shareLinkRepo.GetByToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if link == nil {
		http.Error(w, "Share link not found", http.StatusNotFound)
		return
	}

	file, err := h.fileService.GetFile(link.FileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_name":  file.Name,
		"file_size":  file.Size,
		"mime_type":  file.MimeType,
		"expires_at": link.ExpiresAt,
		"downloads":  link.Downloads,
		"expired":    time.Now().After(link.ExpiresAt),
	})
}

// ListShareLinks 列出文件的分享链接
func (h *FileHandler) ListShareLinks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	links, err := h.shareLinkRepo.GetByFileID(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func (h *FileHandler) ListFolders(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement folder listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

func (h *FileHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement folder creation
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *FileHandler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement folder deletion
	w.WriteHeader(http.StatusNotImplemented)
}

func getQueryParamPtr(r *http.Request, key string) *string {
	val := r.URL.Query().Get(key)
	if val == "" {
		return nil
	}
	return &val
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// renderErrorPage 渲染错误页面
func renderErrorPage(w http.ResponseWriter, title, message string, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>%s - LanSync</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
		.error-container { text-align: center; padding: 2rem; }
		h1 { color: #e74c3c; margin-bottom: 1rem; }
		p { color: #666; margin-bottom: 2rem; }
		a { color: #3498db; text-decoration: none; }
	</style>
</head>
<body>
	<div class="error-container">
		<h1>%s</h1>
		<p>%s</p>
		<a href="/">返回首页</a>
	</div>
</body>
</html>`, title, title, message)))
}

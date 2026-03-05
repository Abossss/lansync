package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/abossss/lansync/internal/services"
)

type APIHandler struct {
	fileService      *services.FileService
	transferService  *services.TransferService
	discoveryService *services.DiscoveryService
}

func NewAPIHandler(fs *services.FileService, ts *services.TransferService, ds *services.DiscoveryService) *APIHandler {
	return &APIHandler{
		fileService:      fs,
		transferService:  ts,
		discoveryService: ds,
	}
}

func (h *APIHandler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement transfer listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

func (h *APIHandler) CancelTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: Implement transfer cancellation
	w.Write([]byte(`{"message": "Transfer ` + id + ` cancelled"}`))
}

// ListPeers 获取其他设备列表
func (h *APIHandler) ListPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.discoveryService == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	peers := h.discoveryService.GetPeers()
	json.NewEncoder(w).Encode(peers)
}

// ListDevices 获取所有设备列表（包括本机）
func (h *APIHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.discoveryService == nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	// 记录访问的客户端
	clientIP := r.RemoteAddr
	userAgent := r.UserAgent()
	h.discoveryService.RecordClient(clientIP, userAgent)

	devices := h.discoveryService.GetDevices()
	json.NewEncoder(w).Encode(devices)
}

// GetLocalDevice 获取本机设备信息
func (h *APIHandler) GetLocalDevice(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.discoveryService == nil {
		json.NewEncoder(w).Encode(nil)
		return
	}

	device := h.discoveryService.GetLocalDevice()
	json.NewEncoder(w).Encode(device)
}

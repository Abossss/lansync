package services

import (
	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/repository"
	"github.com/abossss/lansync/internal/websocket"
)

type TransferService struct {
	cfg         *config.TransferConfig
	sessionRepo *repository.SessionRepository
	hub         *websocket.Hub
}

func NewTransferService(cfg *config.TransferConfig, sessionRepo *repository.SessionRepository, hub *websocket.Hub) *TransferService {
	return &TransferService{
		cfg:         cfg,
		sessionRepo: sessionRepo,
		hub:         hub,
	}
}

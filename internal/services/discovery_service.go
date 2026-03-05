package services

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/models"
	"github.com/abossss/lansync/internal/repository"
)

// DiscoveryMessage 发现消息结构
type DiscoveryMessage struct {
	Type      string `json:"type"`       // "announce" 或 "bye"
	ID        string `json:"id"`         // 设备唯一 ID
	Name      string `json:"name"`       // 设备名称
	Address   string `json:"address"`    // IP 地址
	Port      int    `json:"port"`       // HTTP 端口
	Version   string `json:"version"`    // 版本号
	Timestamp int64  `json:"timestamp"`  // 时间戳
}

// Device 设备信息（包含本机）
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Version   string    `json:"version"`
	LastSeen  time.Time `json:"last_seen"`
	IsLocal   bool      `json:"is_local"`
}

// DiscoveryService 设备发现服务
type DiscoveryService struct {
	cfg       *config.DiscoveryConfig
	peerRepo  *repository.PeerRepository
	localID   string
	localName string
	localPort int
	localIP   string
	conn      *net.UDPConn
	devices   map[string]*Device
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	version   string
}

// NewDiscoveryService 创建发现服务
func NewDiscoveryService(cfg *config.DiscoveryConfig, peerRepo *repository.PeerRepository, localID, localName string, httpPort int) *DiscoveryService {
	return &DiscoveryService{
		cfg:       cfg,
		peerRepo:  peerRepo,
		localID:   localID,
		localName: localName,
		localPort: httpPort,
		devices:   make(map[string]*Device),
		version:   "1.2.0",
	}
}

// Start 启动发现服务
func (d *DiscoveryService) Start() error {
	d.ctx, d.cancel = context.WithCancel(context.Background())

	// 获取本机 IP
	d.localIP = d.getLocalIP()

	// 添加本机设备
	d.addLocalDevice()

	// 加载已知设备
	d.loadKnownDevices()

	if d.cfg.Enabled {
		// 尝试启动 UDP 发现
		if err := d.startUDPDiscovery(); err != nil {
			log.Printf("UDP 发现服务启动失败: %v，将使用 HTTP 发现", err)
		}
	}

	return nil
}

// startUDPDiscovery 启动 UDP 发现
func (d *DiscoveryService) startUDPDiscovery() error {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: d.cfg.Port,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	d.conn = conn

	log.Printf("UDP 发现服务已启动，端口: %d", d.cfg.Port)

	// 启动接收协程
	go d.receive()

	// 启动广播协程
	go d.broadcast()

	// 启动清理协程
	go d.cleanup()

	return nil
}

// addLocalDevice 添加本机设备
func (d *DiscoveryService) addLocalDevice() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.devices[d.localID] = &Device{
		ID:       d.localID,
		Name:     d.localName + " (本机)",
		Address:  d.localIP,
		Port:     d.localPort,
		Version:  d.version,
		LastSeen: time.Now(),
		IsLocal:  true,
	}
}

// loadKnownDevices 从数据库加载已知设备
func (d *DiscoveryService) loadKnownDevices() {
	if d.peerRepo == nil {
		return
	}

	peers, err := d.peerRepo.List()
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, peer := range peers {
		// 跳过本机
		if peer.ID == d.localID {
			continue
		}
		// 跳过回环地址
		if peer.Address == "127.0.0.1" || peer.Address == "::1" ||
			strings.HasPrefix(peer.Address, "127.") ||
			strings.HasPrefix(peer.Address, "::") {
			// 从数据库删除无效记录
			d.peerRepo.Delete(peer.ID)
			continue
		}
		d.devices[peer.ID] = &Device{
			ID:       peer.ID,
			Name:     peer.Name,
			Address:  peer.Address,
			Port:     peer.Port,
			Version:  peer.Version,
			LastSeen: peer.LastSeen,
			IsLocal:  false,
		}
	}
}

// Stop 停止发现服务
func (d *DiscoveryService) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.conn != nil {
		d.sendBye()
		d.conn.Close()
	}
}

// RecordClient 记录访问的客户端设备
func (d *DiscoveryService) RecordClient(clientIP, userAgent string) {
	// 提取 IP（去掉端口）
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 && !strings.Contains(clientIP, ":") {
		// IPv4 with port
		clientIP = clientIP[:idx]
	} else if strings.HasPrefix(clientIP, "[") {
		// IPv6 with port
		if idx := strings.LastIndex(clientIP, "]:"); idx != -1 {
			clientIP = clientIP[1:idx]
		}
	}

	// 跳过本机访问和回环地址
	if clientIP == "127.0.0.1" || clientIP == "::1" || clientIP == d.localIP ||
		strings.HasPrefix(clientIP, "127.") || strings.HasPrefix(clientIP, "::ffff:127") {
		return
	}

	// 跳过 IPv6 回环
	if strings.HasPrefix(clientIP, "::") && len(clientIP) <= 3 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// 查找是否已有此 IP 的设备
	for _, device := range d.devices {
		if device.Address == clientIP && !device.IsLocal {
			device.LastSeen = time.Now()
			if d.peerRepo != nil {
				d.peerRepo.Upsert(&models.Peer{
					ID:        device.ID,
					Name:      device.Name,
					Address:   device.Address,
					Port:      device.Port,
					LastSeen:  device.LastSeen,
					Version:   device.Version,
					FileCount: 0,
				})
			}
			log.Printf("设备更新: %s (%s)", device.Name, device.Address)
			return
		}
	}

	// 新设备
	deviceID := "client-" + strings.ReplaceAll(strings.ReplaceAll(clientIP, ":", "-"), ".", "-")
	deviceName := "LanSync 客户端"

	// 检测是否是移动设备
	if strings.Contains(userAgent, "Mobile") || strings.Contains(userAgent, "Android") {
		deviceName = "移动设备"
	}

	device := &Device{
		ID:       deviceID,
		Name:     deviceName,
		Address:  clientIP,
		Port:     d.localPort, // 假设使用相同端口
		Version:  "unknown",
		LastSeen: time.Now(),
		IsLocal:  false,
	}

	d.devices[deviceID] = device

	// 保存到数据库
	if d.peerRepo != nil {
		d.peerRepo.Upsert(&models.Peer{
			ID:        device.ID,
			Name:      device.Name,
			Address:   device.Address,
			Port:      device.Port,
			LastSeen:  device.LastSeen,
			Version:   device.Version,
			FileCount: 0,
		})
	}

	log.Printf("发现新设备: %s (%s)", deviceName, clientIP)
}

// receive 接收 UDP 广播消息
func (d *DiscoveryService) receive() {
	buf := make([]byte, 1024)
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
			d.conn.SetReadDeadline(time.Now().Add(time.Second))
			n, remoteAddr, err := d.conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if d.ctx.Err() != nil {
					return
				}
				continue
			}

			var msg DiscoveryMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}

			// 忽略自己的消息
			if msg.ID == d.localID {
				continue
			}

			d.handleMessage(&msg, remoteAddr.IP.String())
		}
	}
}

// handleMessage 处理接收到的消息
func (d *DiscoveryService) handleMessage(msg *DiscoveryMessage, ip string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	switch msg.Type {
	case "announce":
		// 使用消息中的地址，如果为空则使用来源 IP
		address := msg.Address
		if address == "" || address == "0.0.0.0" {
			address = ip
		}

		device := &Device{
			ID:       msg.ID,
			Name:     msg.Name,
			Address:  address,
			Port:     msg.Port,
			Version:  msg.Version,
			LastSeen: time.Now(),
			IsLocal:  false,
		}
		d.devices[msg.ID] = device

		// 保存到数据库
		if d.peerRepo != nil {
			d.peerRepo.Upsert(&models.Peer{
				ID:        device.ID,
				Name:      device.Name,
				Address:   device.Address,
				Port:      device.Port,
				LastSeen:  device.LastSeen,
				Version:   device.Version,
				FileCount: 0,
			})
		}

		log.Printf("发现设备: %s (%s:%d)", device.Name, device.Address, device.Port)

	case "bye":
		if _, exists := d.devices[msg.ID]; exists {
			delete(d.devices, msg.ID)
			if d.peerRepo != nil {
				d.peerRepo.Delete(msg.ID)
			}
			log.Printf("设备离线: %s", msg.Name)
		}
	}
}

// broadcast 定期广播
func (d *DiscoveryService) broadcast() {
	// 立即发送几次
	for i := 0; i < 3; i++ {
		d.sendAnnounce()
		time.Sleep(time.Second)
	}

	ticker := time.NewTicker(d.cfg.BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.sendAnnounce()
		}
	}
}

// sendAnnounce 发送公告消息
func (d *DiscoveryService) sendAnnounce() {
	if d.conn == nil {
		return
	}

	msg := DiscoveryMessage{
		Type:      "announce",
		ID:        d.localID,
		Name:      d.localName,
		Address:   d.localIP,
		Port:      d.localPort,
		Version:   d.version,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// 广播到多个地址
	broadcastAddrs := []*net.UDPAddr{
		{IP: net.IPv4(255, 255, 255, 255), Port: d.cfg.Port}, // 广播
		{IP: net.IPv4(224, 0, 0, 1), Port: d.cfg.Port},      // 多播
	}

	// 也发送到局域网子网广播
	if d.localIP != "" {
		parts := strings.Split(d.localIP, ".")
		if len(parts) == 4 {
			subnetBroadcast := parts[0] + "." + parts[1] + "." + parts[2] + ".255"
			ip := net.ParseIP(subnetBroadcast)
			if ip != nil {
				broadcastAddrs = append(broadcastAddrs, &net.UDPAddr{IP: ip, Port: d.cfg.Port})
			}
		}
	}

	for _, addr := range broadcastAddrs {
		d.conn.WriteToUDP(data, addr)
	}
}

// sendBye 发送离开消息
func (d *DiscoveryService) sendBye() {
	if d.conn == nil {
		return
	}

	msg := DiscoveryMessage{
		Type:      "bye",
		ID:        d.localID,
		Name:      d.localName,
		Timestamp: time.Now().Unix(),
	}

	data, _ := json.Marshal(msg)
	broadcastAddr := &net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: d.cfg.Port,
	}
	d.conn.WriteToUDP(data, broadcastAddr)
}

// cleanup 定期清理过期设备
func (d *DiscoveryService) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.mu.Lock()
			for id, device := range d.devices {
				if !device.IsLocal && time.Since(device.LastSeen) > d.cfg.PeerTimeout {
					delete(d.devices, id)
					if d.peerRepo != nil {
						d.peerRepo.Delete(id)
					}
					log.Printf("设备超时移除: %s", device.Name)
				}
			}
			d.mu.Unlock()
		}
	}
}

// GetDevices 获取所有设备列表（包括本机）
func (d *DiscoveryService) GetDevices() []*Device {
	d.mu.RLock()
	defer d.mu.RUnlock()

	devices := make([]*Device, 0, len(d.devices))
	for _, device := range d.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetPeers 获取其他设备列表（不包括本机）
func (d *DiscoveryService) GetPeers() []*models.Peer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	peers := make([]*models.Peer, 0)
	for _, device := range d.devices {
		if !device.IsLocal {
			peers = append(peers, &models.Peer{
				ID:        device.ID,
				Name:      device.Name,
				Address:   device.Address,
				Port:      device.Port,
				LastSeen:  device.LastSeen,
				Version:   device.Version,
				FileCount: 0,
			})
		}
	}
	return peers
}

// GetLocalDevice 获取本机设备信息
func (d *DiscoveryService) GetLocalDevice() *Device {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if device, exists := d.devices[d.localID]; exists {
		return device
	}
	return nil
}

// getLocalIP 获取本机局域网 IP
func (d *DiscoveryService) getLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		hostname, _ := os.Hostname()
		return hostname
	}

	var bestIP string
	var bestPriority int

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()

				// 跳过虚拟网络
				if isVirtualNetwork(ip) {
					continue
				}

				priority := 0
				if strings.HasPrefix(ip, "192.168.") {
					priority = 100
				} else if strings.HasPrefix(ip, "10.") {
					priority = 80
				}

				if priority > bestPriority {
					bestPriority = priority
					bestIP = ip
				}
			}
		}
	}

	return bestIP
}

func isVirtualNetwork(ip string) bool {
	virtualPrefixes := []string{
		"198.18.", "198.19.",
		"169.254.",
		"172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.",
		"172.24.", "172.25.", "172.26.", "172.27.",
		"172.28.", "172.29.", "172.30.", "172.31.",
		"127.",
	}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

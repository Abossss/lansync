package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/handlers"
	"github.com/abossss/lansync/internal/middleware"
	"github.com/abossss/lansync/internal/repository"
	"github.com/abossss/lansync/internal/services"
	"github.com/abossss/lansync/internal/websocket"
	"github.com/gorilla/mux"
)

// setupFirewall 配置 Windows 防火墙规则
func setupFirewall(port int) error {
	// 只在 Windows 上执行
	if runtime.GOOS != "windows" {
		return nil
	}

	ruleName := "LanSync Server"

	// 先删除已存在的规则（避免重复）
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s", ruleName)).Run()

	// 添加 TCP 入站规则
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", ruleName),
		"dir=in",
		"action=allow",
		"protocol=tcp",
		fmt.Sprintf("localport=%d", port),
		"enable=yes",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("添加防火墙规则失败: %v, output: %s", err, string(output))
	}

	// 验证规则是否添加成功
	checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		fmt.Sprintf("name=%s", ruleName))
	verifyOutput, _ := checkCmd.CombinedOutput()

	if !strings.Contains(string(verifyOutput), ruleName) {
		return fmt.Errorf("防火墙规则添加后验证失败")
	}

	log.Printf("已添加防火墙规则 '%s' (端口 %d)", ruleName, port)
	return nil
}

// checkFirewallRule 检查防火墙规则是否存在
func checkFirewallRule(port int) bool {
	if runtime.GOOS != "windows" {
		return true
	}

	ruleName := "LanSync Server"
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		fmt.Sprintf("name=%s", ruleName))
	output, _ := cmd.CombinedOutput()
	return strings.Contains(string(output), ruleName)
}

// checkAdmin 检查是否以管理员权限运行
func checkAdmin() bool {
	if runtime.GOOS != "windows" {
		return true
	}

	_, err := os.Open("\\.\\PHYSICALDRIVE0")
	return err == nil
}

// runAsAdmin 尝试以管理员权限重新运行
func runAsAdmin() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// 使用环境变量标记已经尝试过提权
	os.Setenv("LANSYNC_ADMIN_TRY", "1")

	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Start-Process '%s' -Verb RunAs", execPath))

	return cmd.Run()
}

// hasTriedAdmin 检查是否已经尝试过管理员权限提升
func hasTriedAdmin() bool {
	return os.Getenv("LANSYNC_ADMIN_TRY") == "1"
}

// showNetworkInfo 显示网络诊断信息
func showNetworkInfo() {
	fmt.Println()
	fmt.Println("📋 网络诊断信息:")
	fmt.Println()

	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				status := ""
				if isVirtualNetwork(ip) {
					status = " [虚拟网络]"
				} else if strings.HasPrefix(ip, "192.168.") {
					status = " [局域网 ✓]"
				}
				fmt.Printf("   %s: %s%s\n", iface.Name, ip, status)
			}
		}
	}
}

func setConfigDefaults(cfg *config.Config) {
	// Server defaults
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.ReadTimeout = 30 * time.Second
	cfg.Server.WriteTimeout = 30 * time.Second
	cfg.Server.MaxUpload = 1073741824 // 1GB

	// Storage defaults - use executable directory as base
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	if execDir == "." {
		execDir, _ = os.Getwd()
	}

	cfg.Storage.UploadDir = filepath.Join(execDir, "web", "uploads")
	cfg.Storage.TempDir = filepath.Join(execDir, "web", "uploads", "tmp")
	cfg.Storage.MaxStorage = 10737418240 // 10GB
	cfg.Storage.CleanupInterval = 1 * time.Hour

	// Discovery defaults
	cfg.Discovery.Enabled = true
	cfg.Discovery.Port = 7350
	cfg.Discovery.BroadcastInterval = 30 * time.Second
	cfg.Discovery.PeerTimeout = 5 * time.Minute

	// Transfer defaults
	cfg.Transfer.MaxConcurrent = 5
	cfg.Transfer.ChunkSize = 1048576
	cfg.Transfer.BufferSize = 65536

	// Database defaults
	cfg.Database.Path = filepath.Join(execDir, "lansync.db")

	// UI defaults
	cfg.UI.DefaultTheme = "light"
	cfg.UI.ItemsPerPage = 50
}

func getLocalIP() string {
	// 获取所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var bestIP string
	var bestPriority int

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
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

				// 跳过虚拟网络和特殊地址
				if isVirtualNetwork(ip) {
					continue
				}

				// 计算优先级
				priority := getIPPriority(ip)

				if priority > bestPriority {
					bestPriority = priority
					bestIP = ip
				}
			}
		}
	}

	return bestIP
}

// isVirtualNetwork 检查是否是虚拟网络地址
func isVirtualNetwork(ip string) bool {
	// 常见虚拟网络地址段
	virtualPrefixes := []string{
		"198.18.",   // 基准测试地址
		"198.19.",   // 基准测试地址
		"169.254.",  // 链路本地地址
		"172.16.", "172.17.", "172.18.", "172.19.", // Docker 默认
		"172.20.", "172.21.", "172.22.", "172.23.", // Docker
		"172.24.", "172.25.", "172.26.", "172.27.", // Docker
		"172.28.", "172.29.", "172.30.", "172.31.", // Docker
		"127.",      // 回环
	}

	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(ip, prefix) {
			return true
		}
	}
	return false
}

// getIPPriority 获取 IP 地址优先级
func getIPPriority(ip string) int {
	// 192.168.x.x - 常见家庭/办公室局域网，优先级最高
	if strings.HasPrefix(ip, "192.168.") {
		return 100
	}
	// 10.x.x.x - 企业私有网络，优先级次高
	if strings.HasPrefix(ip, "10.") {
		return 80
	}
	// 其他地址
	return 1
}

func setupLogger(logDir string) (*os.File, error) {
	// Create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(logDir, fmt.Sprintf("lansync_%s.log", timestamp))

	// Open log file
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// Redirect logs to file
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return f, nil
}

func printBanner(accessURL string, localIP string, firewallOK bool) {
	fmt.Println()
	fmt.Println("==============================================")
	fmt.Println("         LanSync - 局域网文件共享工具")
	fmt.Println("==============================================")
	fmt.Println()
	fmt.Println("📡 访问地址:")
	fmt.Printf("   本机: http://localhost:8080\n")
	if localIP != "" {
		accessURL = fmt.Sprintf("http://%s:8080", localIP)
		fmt.Printf("   局域网: %s\n", accessURL)
	}
	fmt.Println()

	// 防火墙状态
	if firewallOK {
		fmt.Println("🛡️  防火墙: 已配置 (局域网可访问)")
	} else {
		fmt.Println("⚠️  防火墙: 未配置 (局域网可能无法访问)")
	}
	fmt.Println()

	// 生成并显示二维码
	if localIP != "" {
		qrCode := generateQRCode(accessURL)
		fmt.Println("📱 手机扫码访问:")
		fmt.Println(qrCode)
		fmt.Println()
	}

	fmt.Println("💡 使用提示:")
	fmt.Println("   - 按 Ctrl+C 停止服务器")
	fmt.Println("   - 日志保存在 logs/ 目录")
	fmt.Println()
	fmt.Println("==============================================")
	fmt.Println()
}

func generateQRCode(text string) string {
	// 生成二维码
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return fmt.Sprintf("二维码生成失败: %v", err)
	}

	// 转换为ASCII字符
	return qr.ToSmallString(false)
}

func main() {
	// Get executable directory to resolve relative paths
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("警告: 无法获取可执行文件路径: %v\n", err)
		execPath = "."
	}
	execDir := filepath.Dir(execPath)

	// Setup logging
	logDir := filepath.Join(execDir, "logs")
	logFile, err := setupLogger(logDir)
	if err != nil {
		fmt.Printf("警告: 无法创建日志文件: %v\n", err)
		fmt.Println("日志将输出到控制台")
	} else {
		defer logFile.Close()
		log.Printf("日志文件: %s", logFile.Name())
	}

	// Try to find config file in multiple locations
	configPaths := []string{
		filepath.Join(execDir, "config", "config.yaml"),
		"config/config.yaml",
		"config.yaml",
	}

	var cfg *config.Config
	configErr := error(nil)
	for _, configPath := range configPaths {
		cfg, err = config.Load(configPath)
		if err == nil {
			log.Printf("配置文件: %s", configPath)
			break
		}
		configErr = err
	}

	// If all config paths failed, use defaults
	if cfg == nil {
		log.Printf("警告: 无法加载配置文件 (%v)，使用默认配置", configErr)
		cfg = &config.Config{}
		// Set defaults directly
		setConfigDefaults(cfg)
	}

	// Convert relative paths to absolute paths
	// 使用可执行文件目录作为基准
	uploadDir := cfg.Storage.UploadDir
	tempDir := cfg.Storage.TempDir
	dbPath := cfg.Database.Path

	if !filepath.IsAbs(uploadDir) {
		uploadDir = filepath.Join(execDir, uploadDir)
	}
	if !filepath.IsAbs(tempDir) {
		tempDir = filepath.Join(execDir, tempDir)
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(execDir, dbPath)
	}

	// 更新配置为绝对路径
	cfg.Storage.UploadDir = uploadDir
	cfg.Storage.TempDir = tempDir
	cfg.Database.Path = dbPath

	log.Printf("上传目录: %s", uploadDir)
	log.Printf("临时目录: %s", tempDir)
	log.Printf("数据库: %s", dbPath)

	// Create uploads directory
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("无法创建上传目录: %v", err)
	}
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("无法创建临时目录: %v", err)
	}

	// Initialize database
	db, err := repository.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := repository.RunMigrations(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// Initialize repositories
	fileRepo := repository.NewFileRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	shareLinkRepo := repository.NewShareLinkRepository(db)
	peerRepo := repository.NewPeerRepository(db)

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize services
	fileService := services.NewFileService(cfg, fileRepo, sessionRepo, hub)
	transferService := services.NewTransferService(&cfg.Transfer, sessionRepo, hub)

	// Initialize discovery service
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "LanSync Device"
	}
	localID := fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()%10000)
	discoveryService := services.NewDiscoveryService(&cfg.Discovery, peerRepo, localID, hostname, cfg.Server.Port)
	if err := discoveryService.Start(); err != nil {
		log.Printf("设备发现服务启动失败: %v", err)
	}
	defer discoveryService.Stop()

	// Initialize handlers
	fileHandler := handlers.NewFileHandler(fileService, cfg, shareLinkRepo)
	apiHandler := handlers.NewAPIHandler(fileService, transferService, discoveryService)
	templateHandler := handlers.NewTemplateHandler(cfg, fileService)

	// Setup router
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/upload", fileHandler.UploadFile).Methods("POST")
	api.HandleFunc("/files", fileHandler.ListFiles).Methods("GET")
	api.HandleFunc("/files/search", fileHandler.SearchFiles).Methods("GET")
	api.HandleFunc("/files/{id}", fileHandler.GetFile).Methods("GET")
	api.HandleFunc("/files/{id}", fileHandler.DeleteFile).Methods("DELETE")
	api.HandleFunc("/download/{id}", fileHandler.DownloadFile).Methods("GET")
	api.HandleFunc("/preview/{id}", fileHandler.PreviewFile).Methods("GET")
	api.HandleFunc("/batch-download", fileHandler.BatchDownload).Methods("POST")
	api.HandleFunc("/storage", fileHandler.GetStorageStats).Methods("GET")
	api.HandleFunc("/folders", fileHandler.ListFolders).Methods("GET")
	api.HandleFunc("/folders", fileHandler.CreateFolder).Methods("POST")
	api.HandleFunc("/folders/{id}", fileHandler.DeleteFolder).Methods("DELETE")
	api.HandleFunc("/transfers", apiHandler.ListTransfers).Methods("GET")
	api.HandleFunc("/transfers/{id}", apiHandler.CancelTransfer).Methods("DELETE")
	api.HandleFunc("/peers", apiHandler.ListPeers).Methods("GET")
	api.HandleFunc("/devices", apiHandler.ListDevices).Methods("GET")
	api.HandleFunc("/device/local", apiHandler.GetLocalDevice).Methods("GET")

	// Share API routes
	api.HandleFunc("/share/{id}", fileHandler.CreateShareLink).Methods("POST")
	api.HandleFunc("/share/{id}/links", fileHandler.ListShareLinks).Methods("GET")

	// WebSocket routes
	router.HandleFunc("/ws/progress", hub.HandleWebSocket)

	// Share page routes (public)
	router.HandleFunc("/share/{token}", fileHandler.DownloadByShare).Methods("GET")
	router.HandleFunc("/share/{token}/info", fileHandler.GetShareInfo).Methods("GET")

	// Static files
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Page routes
	router.HandleFunc("/", templateHandler.HomePage).Methods("GET")
	router.HandleFunc("/upload", templateHandler.UploadPage).Methods("GET")
	router.HandleFunc("/browse", templateHandler.BrowsePage).Methods("GET")
	router.HandleFunc("/downloads", templateHandler.DownloadsPage).Methods("GET")
	router.HandleFunc("/peers", templateHandler.PeersPage).Methods("GET")
	router.HandleFunc("/settings", templateHandler.SettingsPage).Methods("GET")

	// Get local IP for status endpoint
	localIP := getLocalIP()

	// API status endpoint
	router.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := map[string]interface{}{
			"status":    "running",
			"version":   "1.1.0",
			"localIP":   localIP,
			"port":      cfg.Server.Port,
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		json.NewEncoder(w).Encode(status)
	}).Methods("GET")

	// Apply middleware
	router.Use(middleware.Logging)
	router.Use(middleware.Recovery)
	router.Use(middleware.CORS)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Configure firewall for LAN access
	firewallOK := checkFirewallRule(cfg.Server.Port)
	fmt.Println()

	if !firewallOK {
		fmt.Println("🔧 配置防火墙...")
		if checkAdmin() {
			// 已有管理员权限，直接配置防火墙
			if err := setupFirewall(cfg.Server.Port); err != nil {
				fmt.Printf("❌ 防火墙配置失败: %v\n", err)
				fmt.Println()
			} else {
				fmt.Println("✅ 防火墙配置成功")
				firewallOK = true
			}
		} else if !hasTriedAdmin() {
			// 没有管理员权限，且未尝试过提权，尝试提权
			fmt.Println("⚠️  当前未以管理员权限运行")
			fmt.Println("   正在请求管理员权限（请点击'是'）...")
			if err := runAsAdmin(); err != nil {
				fmt.Printf("   请求失败: %v\n", err)
			} else {
				// 成功启动了管理员进程，退出当前进程
				fmt.Println("   已在新窗口中启动...")
				os.Exit(0)
			}
		}
	} else {
		fmt.Println("✅ 防火墙已配置")
	}

	// 显示网络信息
	showNetworkInfo()

	// Start server in goroutine
	go func() {
		log.Printf("服务器启动: %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Check if it's a port binding error
			if strings.Contains(err.Error(), "Only one usage of each socket address") ||
				strings.Contains(err.Error(), "address already in use") ||
				strings.Contains(err.Error(), "bind:") {
				fmt.Println()
				fmt.Println("==============================================")
				fmt.Printf("  错误: 端口 %d 已被占用\n", cfg.Server.Port)
				fmt.Println("==============================================")
				fmt.Println()
				fmt.Println("可能的解决方案:")
				fmt.Printf("1. 关闭其他使用端口 %d 的程序\n", cfg.Server.Port)
				fmt.Println("2. 修改配置文件 config/config.yaml 中的端口号")
				fmt.Println()
				fmt.Println("查找占用端口的程序:")
				fmt.Printf("  Windows: netstat -ano | findstr :%d\n", cfg.Server.Port)
				fmt.Println("==============================================")
				fmt.Println()
			} else {
				log.Printf("服务器启动失败: %v", err)
			}
		}
	}()

	// Print banner with access URLs
	printBanner("", localIP, firewallOK)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println()
	fmt.Println("正在停止服务器...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务器强制关闭: %v", err)
	}

	log.Println("服务器已退出")
	fmt.Println("服务器已停止")
}

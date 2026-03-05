package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/abossss/lansync/internal/config"
	"github.com/abossss/lansync/internal/services"
)

type TemplateHandler struct {
	cfg         *config.Config
	fileService *services.FileService
}

func NewTemplateHandler(cfg *config.Config, fs *services.FileService) *TemplateHandler {
	return &TemplateHandler{
		cfg:         cfg,
		fileService: fs,
	}
}

func (h *TemplateHandler) renderTemplate(w http.ResponseWriter, tmpl, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="zh-CN" data-theme="` + h.cfg.UI.DefaultTheme + `">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>LanSync - ` + tmpl + `</title>
	<link rel="stylesheet" href="/static/css/main.css">
	<link rel="stylesheet" href="/static/css/themes.css">
	<link rel="stylesheet" href="/static/css/components.css">
</head>
<body class="theme-` + h.cfg.UI.DefaultTheme + `">
	<div id="app">
		<nav class="navbar">
			<div class="nav-container">
				<div class="nav-brand">
					<a href="/" style="text-decoration: none; color: inherit;">
						<h1>🚀 LanSync</h1>
					</a>
				</div>
				<ul class="nav-menu">
					<li><a href="/">🏠 首页</a></li>
					<li><a href="/upload">📤 上传</a></li>
					<li><a href="/browse">📁 浏览</a></li>
					<li><a href="/downloads">⬇️ 下载</a></li>
					<li><a href="/peers">🌐 设备</a></li>
				</ul>
				<button id="theme-toggle" class="theme-toggle" title="切换主题">
					<span class="icon">🌓</span>
				</button>
			</div>
		</nav>
		<main class="main-content">
			` + content + `
		</main>
		<footer style="text-align: center; padding: 2rem; color: var(--text-secondary); border-top: 1px solid var(--border-color); margin-top: 3rem;">
			<p>&copy; 2025 LanSync - 局域网文件共享工具</p>
		</footer>
	</div>
	<script src="/static/js/theme.js"></script>
</body>
</html>`))
}

func (h *TemplateHandler) HomePage(w http.ResponseWriter, r *http.Request) {
	// 获取存储统计
	stats, err := h.fileService.GetStorageStatsDetail()
	if err != nil || stats == nil {
		// 使用默认值
		stats = &services.StorageStats{
			UsedBytes:   0,
			TotalBytes:  10737418240,
			UsedPercent: 0,
			FileCount:   0,
		}
	}

	content := `
		<div class="text-center" style="padding: 3rem 1rem;">
			<h1 style="font-size: 2.5rem; margin-bottom: 1rem; color: var(--accent-color);">欢迎使用 LanSync</h1>
			<p style="font-size: 1.25rem; color: var(--text-secondary); margin-bottom: 2rem;">
				快速、安全的局域网文件共享工具
			</p>
			<div style="display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap;">
				<a href="/upload" class="btn btn-primary">📤 上传文件</a>
				<a href="/browse" class="btn btn-secondary">📁 浏览文件</a>
				<a href="/peers" class="btn btn-secondary">🌐 发现设备</a>
			</div>
		</div>

		<!-- 存储统计 -->
		<div class="card" style="margin: 2rem 0;">
			<h3 style="margin-bottom: 1rem;">📊 存储空间</h3>
			<div id="storage-stats">
				<div style="display: flex; justify-content: space-between; margin-bottom: 0.5rem;">
					<span>已使用: <strong id="used-space">` + formatSize(stats.UsedBytes) + `</strong></span>
					<span>总空间: <strong id="total-space">` + formatSize(stats.TotalBytes) + `</strong></span>
				</div>
				<div class="progress-bar" style="height: 12px; border-radius: 6px;">
					<div class="progress-fill" id="storage-bar" style="width: ` + strconv.Itoa(stats.UsedPercent) + `%; background: ` + getProgressColor(stats.UsedPercent) + `;"></div>
				</div>
				<p style="text-align: center; margin-top: 0.5rem; color: var(--text-secondary);">
					已存储 <strong>` + strconv.Itoa(stats.FileCount) + `</strong> 个文件
				</p>
			</div>
		</div>

		<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1.5rem; margin-top: 3rem;">
			<div class="card">
				<h3 style="margin-bottom: 0.5rem;">🚀 快速传输</h3>
				<p style="color: var(--text-secondary);">支持大文件高速传输，断点续传功能</p>
			</div>
			<div class="card">
				<h3 style="margin-bottom: 0.5rem;">🔗 分享链接</h3>
				<p style="color: var(--text-secondary);">生成临时分享链接，安全分享文件</p>
			</div>
			<div class="card">
				<h3 style="margin-bottom: 0.5rem;">👁️ 文件预览</h3>
				<p style="color: var(--text-secondary);">图片、PDF、文本在线预览</p>
			</div>
			<div class="card">
				<h3 style="margin-bottom: 0.5rem;">📦 批量下载</h3>
				<p style="color: var(--text-secondary);">多文件打包 ZIP 一键下载</p>
			</div>
		</div>
	`
	h.renderTemplate(w, "Home", content)
}

func formatSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for bytes >= unit && i < len(sizes)-1 {
		bytes /= unit
		i++
	}
	return fmt.Sprintf("%d %s", bytes, sizes[i])
}

func getProgressColor(percent int) string {
	if percent < 50 {
		return "#4CAF50"
	} else if percent < 80 {
		return "#FF9800"
	}
	return "#F44336"
}

func (h *TemplateHandler) UploadPage(w http.ResponseWriter, r *http.Request) {
	content := `
		<div class="upload-container">
			<h1 style="text-align: center; margin-bottom: 2rem;">上传文件</h1>

			<form id="upload-form">
				<div id="drop-zone">
					<div class="drop-zone-content">
						<div class="drop-zone-icon">📁</div>
						<div class="drop-zone-text">拖放文件到这里</div>
						<div class="drop-zone-hint">或点击选择文件</div>
					</div>
					<input type="file" id="file-input" multiple>
				</div>
			</form>

			<div id="uploads-list"></div>
		</div>

		<script src="/static/js/upload.js"></script>
	`
	h.renderTemplate(w, "Upload", content)
}

func (h *TemplateHandler) BrowsePage(w http.ResponseWriter, r *http.Request) {
	content := `
		<h1>浏览文件</h1>

		<div class="file-browser">
			<div class="file-toolbar">
				<div class="file-search">
					<input type="text" id="search-input" placeholder="搜索文件...">
					<span class="file-search-icon">🔍</span>
				</div>
				<button class="btn btn-secondary" onclick="refreshFiles()">🔄 刷新</button>
			</div>

			<div id="file-grid" class="file-grid">
				<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;">
					<div class="spinner"></div>
					<p style="color: var(--text-secondary);">加载中...</p>
				</div>
			</div>
		</div>

		<script>
			async function loadFiles() {
				const grid = document.getElementById('file-grid');
				try {
					const response = await fetch('/api/files?limit=50');
					const files = await response.json();

					if (files.length === 0) {
						grid.innerHTML = '<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;"><p style="color: var(--text-secondary); font-size: 1.125rem;">📭 暂无文件</p><a href="/upload" class="btn btn-primary" style="margin-top: 1rem;">上传第一个文件</a></div>';
						return;
					}

					grid.innerHTML = files.map(function(file) {
						return '<div class="file-card" onclick="downloadFile(\'' + file.id + '\')"><div class="file-icon">' + getFileIcon(file.name) + '</div><div class="file-name" title="' + file.name + '">' + file.name + '</div><div class="file-meta"><span>' + formatSize(file.size) + '</span><span>' + new Date(file.created_at).toLocaleDateString() + '</span></div><div class="file-actions"><button class="btn btn-primary" onclick="event.stopPropagation(); downloadFile(\'' + file.id + '\')">⬇️ 下载</button><button class="btn btn-secondary" onclick="event.stopPropagation(); deleteFile(\'' + file.id + '\', \'' + file.name + '\')">🗑️ 删除</button></div></div>';
					}).join('');
				} catch (error) {
					grid.innerHTML = '<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;"><p style="color: var(--danger);">加载失败: ' + error.message + '</p><button class="btn btn-secondary" onclick="loadFiles()" style="margin-top: 1rem;">重试</button></div>';
				}
			}

			function getFileIcon(filename) {
				const ext = filename.split('.').pop().toLowerCase();
				const icons = {
					'pdf': '📄',
					'doc': '📝', 'docx': '📝',
					'xls': '📊', 'xlsx': '📊',
					'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️', 'gif': '🖼️',
					'mp3': '🎵', 'wav': '🎵',
					'mp4': '🎬', 'avi': '🎬', 'mkv': '🎬',
					'zip': '📦', 'rar': '📦', '7z': '📦'
				};
				return icons[ext] || '📄';
			}

			function formatSize(bytes) {
				if (bytes === 0) return '0 B';
				const k = 1024;
				const sizes = ['B', 'KB', 'MB', 'GB'];
				const i = Math.floor(Math.log(bytes) / Math.log(k));
				return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
			}

			function downloadFile(id) {
				window.location.href = '/api/download/' + id;
			}

			async function deleteFile(id, name) {
				if (!confirm('确定要删除 "' + name + '" 吗？')) return;

				try {
					const response = await fetch('/api/files/' + id, { method: 'DELETE' });
					if (response.ok) {
						loadFiles();
					} else {
						alert('删除失败');
					}
				} catch (error) {
					alert('删除失败: ' + error.message);
				}
			}

			function refreshFiles() {
				loadFiles();
			}

			// Load files on page load
			loadFiles();
		</script>
	`
	h.renderTemplate(w, "Browse", content)
}

func (h *TemplateHandler) DownloadsPage(w http.ResponseWriter, r *http.Request) {
	content := `
		<h1>下载管理</h1>
		<div class="card">
			<p style="color: var(--text-secondary); text-align: center; padding: 2rem;">暂无活动下载</p>
		</div>
	`
	h.renderTemplate(w, "Downloads", content)
}

func (h *TemplateHandler) PeersPage(w http.ResponseWriter, r *http.Request) {
	content := `
		<h1>发现设备</h1>

		<div class="file-toolbar" style="margin-bottom: 1.5rem;">
			<p style="color: var(--text-secondary);">局域网中运行 LanSync 的设备</p>
			<button class="btn btn-primary" onclick="loadDevices()">🔄 刷新列表</button>
		</div>

		<div id="devices-list" class="file-grid">
			<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;">
				<div class="spinner"></div>
				<p style="color: var(--text-secondary);">正在扫描...</p>
			</div>
		</div>

		<div style="margin-top: 2rem; padding: 1rem; background: var(--bg-secondary); border-radius: 8px;">
			<h4 style="margin-bottom: 0.5rem;">💡 使用提示</h4>
			<ul style="color: var(--text-secondary); font-size: 0.875rem; margin: 0; padding-left: 1.5rem;">
				<li>本机设备会显示 "(本机)" 标记</li>
				<li>其他设备访问本机时会自动被发现</li>
				<li>运行 LanSync 的设备会自动互相发现</li>
			</ul>
		</div>

		<script>
			async function loadDevices() {
				const list = document.getElementById('devices-list');
				try {
					const response = await fetch('/api/devices');
					const devices = await response.json();

					if (devices.length === 0) {
						list.innerHTML = '<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;"><p style="color: var(--text-secondary);">📭 未发现设备</p></div>';
						return;
					}

					// 按本机优先排序
					devices.sort(function(a, b) {
						if (a.is_local) return -1;
						if (b.is_local) return 1;
						return 0;
					});

					list.innerHTML = devices.map(function(device) {
						var icon = device.is_local ? '🖥️' : '💻';
						var nameDisplay = device.name;
						if (device.is_local) {
							return '<div class="file-card" style="border-color: var(--accent-color);"><div class="file-icon">' + icon + '</div><div class="file-name" title="' + device.name + '">' + nameDisplay + '</div><div class="file-meta"><span>' + device.address + ':' + device.port + '</span><span style="color: var(--success);">● 在线</span></div><div class="file-actions"><span style="color: var(--text-muted);">本机</span></div></div>';
						} else {
							return '<div class="file-card" onclick="openDevice(\'' + device.address + '\', ' + device.port + ')"><div class="file-icon">' + icon + '</div><div class="file-name" title="' + device.name + '">' + nameDisplay + '</div><div class="file-meta"><span>' + device.address + ':' + device.port + '</span><span>' + formatTime(device.last_seen) + '</span></div><div class="file-actions"><button class="btn btn-primary">🔗 访问</button></div></div>';
						}
					}).join('');
				} catch (error) {
					list.innerHTML = '<div class="text-center" style="grid-column: 1 / -1; padding: 3rem;"><p style="color: var(--danger);">加载失败: ' + error.message + '</p></div>';
				}
			}

			function openDevice(address, port) {
				window.open('http://' + address + ':' + port, '_blank');
			}

			function formatTime(timeStr) {
				var date = new Date(timeStr);
				var now = new Date();
				var diff = Math.floor((now - date) / 1000);
				if (diff < 60) return '刚刚';
				if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
				if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
				return date.toLocaleDateString();
			}

			loadDevices();
			// 每 30 秒刷新一次
			setInterval(loadDevices, 30000);
		</script>
	`
	h.renderTemplate(w, "Peers", content)
}

func (h *TemplateHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	content := `
		<h1>设置</h1>
		<div class="card">
			<p style="color: var(--text-secondary);">设置页面即将推出</p>
		</div>
	`
	h.renderTemplate(w, "Settings", content)
}

# LanSync

<div align="center">

**轻量级局域网文件共享工具**

无需安装，即开即用，手机扫码即可传输文件

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey)]()

</div>

---

## 简介

LanSync 是一个专为局域网设计的文件共享工具。当你需要在电脑和手机之间传输文件，或者在小团队内共享资料时，只需启动程序，扫描二维码即可开始传输，无需登录注册，无需复杂配置。

**适用场景：**
- 电脑与手机之间互传文件
- 局域网内小团队文件共享
- 临时文件分发与收集
- 替代 U 盘/即时通讯软件传文件

## 功能特性

| 功能 | 描述 |
|------|------|
| 🚀 **快速传输** | 支持大文件上传下载，实时进度显示 |
| 📱 **扫码访问** | 自动生成二维码，手机扫码即可访问 |
| 🌓 **主题切换** | 支持明/暗两种主题，护眼舒适 |
| 🔍 **文件搜索** | 快速检索已上传的文件 |
| 📦 **批量下载** | 多文件打包为 ZIP 一键下载 |
| 🔗 **分享链接** | 生成临时分享链接，可设置过期时间和下载次数 |
| 🖥️ **设备发现** | 自动发现局域网内其他运行 LanSync 的设备 |
| 🛡️ **安全可靠** | 支持 Windows 防火墙自动配置 |

## 技术栈

| 类别 | 技术 |
|------|------|
| 后端 | Go 1.21+ / Gorilla Mux / WebSocket |
| 数据库 | SQLite (modernc.org/sqlite) |
| 前端 | 原生 HTML / CSS / JavaScript |
| 配置 | YAML / Viper |

## 快速开始

### 方式一：直接运行（推荐）

从 [Releases](https://github.com/Abossss/lansync/releases) 下载对应平台的可执行文件：

- Windows: 下载 `lansync-windows-amd64.exe`，双击运行
- Linux: 下载 `lansync-linux-amd64`，`chmod +x` 后运行
- macOS: 下载 `lansync-darwin-amd64`，`chmod +x` 后运行

### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/Abossss/lansync.git
cd lansync

# 安装依赖
go mod download

# 编译运行
go build -o lansync.exe ./cmd/lansync
./lansync.exe
```

### 访问服务

程序启动后会显示访问地址和二维码：

```
==============================================
         LanSync - 局域网文件共享工具
==============================================

📡 访问地址:
   本机: http://localhost:8080
   局域网: http://192.168.1.100:8080

📱 手机扫码访问:
   [二维码]

==============================================
```

## 使用指南

### 上传文件

1. 点击顶部导航「上传」
2. 拖拽文件到上传区域，或点击选择文件
3. 支持同时上传多个文件
4. 实时查看上传进度

### 下载文件

1. 点击顶部导航「浏览」
2. 找到目标文件，点击下载按钮
3. 支持勾选多个文件批量下载（自动打包 ZIP）

### 创建分享链接

1. 在文件列表中点击「分享」按钮
2. 设置过期时间（默认 24 小时）
3. 设置最大下载次数（可选）
4. 将生成的链接发送给对方

## 配置说明

配置文件位于 `config/config.yaml`：

```yaml
# 服务配置
server:
  host: "0.0.0.0"           # 监听地址
  port: 8080                 # 监听端口
  read_timeout: 30s          # 读取超时
  write_timeout: 30s         # 写入超时
  max_upload_size: 1073741824  # 最大上传文件 (1GB)

# 存储配置
storage:
  upload_dir: "./web/uploads"    # 上传目录
  temp_dir: "./web/uploads/tmp"  # 临时目录
  max_storage: 10737418240       # 最大存储空间 (10GB)
  cleanup_interval: 1h           # 清理间隔

# 设备发现
discovery:
  enabled: true              # 是否启用
  port: 7350                 # UDP 广播端口
  broadcast_interval: 30s    # 广播间隔
  peer_timeout: 5m           # 设备超时

# 界面配置
ui:
  default_theme: "light"     # 默认主题 (light/dark)
  items_per_page: 50         # 每页显示数量
```

## 项目结构

```
LanSync/
├── cmd/lansync/main.go      # 程序入口
├── internal/
│   ├── config/              # 配置管理
│   ├── handlers/            # HTTP 处理器
│   ├── services/            # 业务逻辑
│   ├── repository/          # 数据访问
│   ├── middleware/          # 中间件
│   ├── models/              # 数据模型
│   └── websocket/           # WebSocket 通信
├── web/
│   ├── static/              # 静态资源 (CSS/JS)
│   ├── templates/           # HTML 模板
│   └── uploads/             # 上传文件存储
├── config/config.yaml       # 配置文件
├── go.mod                   # Go 模块定义
└── README.md
```

## API 接口

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/upload` | 上传文件 |
| GET | `/api/files` | 获取文件列表 |
| GET | `/api/files/{id}` | 获取文件信息 |
| DELETE | `/api/files/{id}` | 删除文件 |
| GET | `/api/download/{id}` | 下载文件 |
| GET | `/api/preview/{id}` | 预览文件 |
| POST | `/api/batch-download` | 批量下载 |
| GET | `/api/storage` | 存储统计 |
| POST | `/api/share/{id}` | 创建分享链接 |
| GET | `/api/peers` | 获取设备列表 |
| GET | `/ws/progress` | WebSocket 进度推送 |

## 开发

```bash
# 运行开发模式
go run ./cmd/lansync

# 运行测试
go test ./...

# 构建发布版本
go build -ldflags="-s -w" -o lansync ./cmd/lansync
```

## 常见问题

<details>
<summary><b>程序启动后手机无法访问？</b></summary>

1. 确保电脑和手机在同一个局域网
2. 检查 Windows 防火墙是否允许程序通过
3. 尝试以管理员权限运行程序（会自动配置防火墙）
</details>

<details>
<summary><b>上传大文件失败？</b></summary>

1. 检查配置文件中的 `max_upload_size` 设置
2. 确保磁盘空间充足
3. 检查网络连接是否稳定
</details>

<details>
<summary><b>如何修改默认端口？</b></summary>

编辑 `config/config.yaml`，修改 `server.port` 值，然后重启程序。
</details>

## 贡献

欢迎提交 Issue 和 Pull Request。

## 许可证

[MIT License](LICENSE)

---

<div align="center">

Made with ❤️ by [Abossss](https://github.com/Abossss)

</div>

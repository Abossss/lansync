package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Transfer  TransferConfig  `mapstructure:"transfer"`
	Database  DatabaseConfig  `mapstructure:"database"`
	UI        UIConfig        `mapstructure:"ui"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	MaxUpload    int64         `mapstructure:"max_upload_size"`
}

type StorageConfig struct {
	UploadDir       string        `mapstructure:"upload_dir"`
	TempDir         string        `mapstructure:"temp_dir"`
	MaxStorage      int64         `mapstructure:"max_storage"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type DiscoveryConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Port              int           `mapstructure:"port"`
	BroadcastInterval time.Duration `mapstructure:"broadcast_interval"`
	PeerTimeout       time.Duration `mapstructure:"peer_timeout"`
}

type TransferConfig struct {
	MaxConcurrent int `mapstructure:"max_concurrent"`
	ChunkSize     int `mapstructure:"chunk_size"`
	BufferSize    int `mapstructure:"buffer_size"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type UIConfig struct {
	DefaultTheme string `mapstructure:"default_theme"`
	ItemsPerPage int    `mapstructure:"items_per_page"`
}

func Load(path string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		// If config file doesn't exist, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Unmarshal
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.max_upload_size", 1073741824) // 1GB

	// Storage defaults
	v.SetDefault("storage.upload_dir", "./web/uploads")
	v.SetDefault("storage.temp_dir", "./web/uploads/tmp")
	v.SetDefault("storage.max_storage", 10737418240) // 10GB
	v.SetDefault("storage.cleanup_interval", 1*time.Hour)

	// Discovery defaults
	v.SetDefault("discovery.enabled", true)
	v.SetDefault("discovery.port", 7350)
	v.SetDefault("discovery.broadcast_interval", 30*time.Second)
	v.SetDefault("discovery.peer_timeout", 5*time.Minute)

	// Transfer defaults
	v.SetDefault("transfer.max_concurrent", 5)
	v.SetDefault("transfer.chunk_size", 1048576) // 1MB
	v.SetDefault("transfer.buffer_size", 65536)  // 64KB

	// Database defaults
	v.SetDefault("database.path", "./lansync.db")

	// UI defaults
	v.SetDefault("ui.default_theme", "light")
	v.SetDefault("ui.items_per_page", 50)
}

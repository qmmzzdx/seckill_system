package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig 定义服务器相关配置
type ServerConfig struct {
	Port int `yaml:"port"` // 服务监听端口
}

// MysqlConfig 定义MySQL数据库连接配置
type MysqlConfig struct {
	Host     string `yaml:"host"`     // 数据库主机地址
	Port     int    `yaml:"port"`     // 数据库端口
	User     string `yaml:"user"`     // 数据库用户名
	Password string `yaml:"password"` // 数据库密码
	Name     string `yaml:"name"`     // 数据库名称
}

// RedisConfig 定义Redis集群配置
type RedisConfig struct {
	ClusterNodes string `yaml:"cluster_nodes"` // Redis集群节点地址，多个节点用逗号分隔
	Password     string `yaml:"password"`      // Redis访问密码
}

// KafkaConfig 定义Kafka消息队列配置
type KafkaConfig struct {
	Brokers string `yaml:"brokers"`  // Kafka broker地址，多个用逗号分隔
	Topic   string `yaml:"topic"`    // Kafka主题名称
	GroupID string `yaml:"group_id"` // 消费者组ID
}

// EtcdConfig 定义Etcd配置
type EtcdConfig struct {
	Host        string `yaml:"host"`         // Etcd服务地址
	DialTimeout int    `yaml:"dial_timeout"` // 连接超时时间（秒）
	Username    string `yaml:"username"`     // 认证用户名
	Password    string `yaml:"password"`     // 认证密码
}

// LogConfig 定义日志配置
type LogConfig struct {
	Level    string `yaml:"level"`     // 日志级别
	FilePath string `yaml:"file_path"` // 日志文件路径
	MaxSize  int64  `yaml:"max_size"`  // 单个日志文件最大大小（MB）
}

// Config 聚合所有配置项
type Config struct {
	Server      ServerConfig `yaml:"server"`      // 服务器配置
	Database    MysqlConfig  `yaml:"database"`    // MySQL数据库配置
	Redis       RedisConfig  `yaml:"redis"`       // Redis配置
	Kafka       KafkaConfig  `yaml:"kafka"`       // Kafka配置
	Etcd        EtcdConfig   `yaml:"etcd"`        // Etcd配置
	Log         LogConfig    `yaml:"log"`         // 日志配置
	Environment string       `yaml:"environment"` // 运行环境
}

// AppConfig 全局配置实例
var AppConfig *Config

// GetRedisClusterNodes 将Redis集群节点字符串转换为切片
func (rc *RedisConfig) GetRedisClusterNodes() []string {
	return strings.Split(rc.ClusterNodes, ",")
}

// GetKafkaBrokers 将Kafka broker地址字符串转换为切片
func (kc *KafkaConfig) GetKafkaBrokers() []string {
	return strings.Split(kc.Brokers, ",")
}

// GetEtcdEndpoints 获取Etcd服务端点（返回切片形式）
func (ec *EtcdConfig) GetEtcdEndpoints() []string {
	return []string{ec.Host}
}

// Validate 验证配置完整性
func (cfg *Config) Validate() error {
	// 服务器端口验证：确保端口在有效范围内（1-65535）
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

	// 数据库配置验证：检查必需的主机、端口、用户名和数据库名
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535, got %d", cfg.Database.Port)
	}
	if cfg.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if cfg.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}

	// Redis配置验证：确保集群节点配置不为空且有效
	if cfg.Redis.ClusterNodes == "" {
		return fmt.Errorf("redis cluster nodes are required")
	}
	nodes := cfg.Redis.GetRedisClusterNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no valid redis cluster nodes found")
	}

	// Kafka配置验证：检查broker地址和主题配置
	if cfg.Kafka.Brokers == "" {
		return fmt.Errorf("kafka brokers are required")
	}
	brokers := cfg.Kafka.GetKafkaBrokers()
	if len(brokers) == 0 {
		return fmt.Errorf("no valid kafka brokers found")
	}
	if cfg.Kafka.Topic == "" {
		return fmt.Errorf("kafka topic is required")
	}

	// Etcd配置验证：确保主机地址和超时时间有效
	if cfg.Etcd.Host == "" {
		return fmt.Errorf("etcd host is required")
	}
	if cfg.Etcd.DialTimeout <= 0 {
		return fmt.Errorf("etcd dial timeout must be positive")
	}

	// 日志配置验证和默认值设置
	if cfg.Log.MaxSize <= 0 {
		cfg.Log.MaxSize = 20 // 默认日志文件大小为20MB
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info" // 默认日志级别为info
	}
	if cfg.Log.FilePath == "" {
		cfg.Log.FilePath = "logs" // 默认日志目录为logs
	}

	return nil
}

// InitConfig 从指定路径加载YAML配置文件
func InitConfig(path string) error {
	// 读取配置文件：使用os.ReadFile读取整个文件内容到内存
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析YAML配置：使用yaml.v3库将YAML内容反序列化为Config结构体
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// 配置验证：调用Validate方法检查所有必需配置项
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %v", err)
	}

	// 设置全局配置：将解析后的配置赋值给包级全局变量
	AppConfig = &cfg

	// 初始化日志系统：设置slog默认logger，包含控制台和文件输出
	if err := initLogger(); err != nil {
		return fmt.Errorf("failed to initialize logger: %v", err)
	}

	// 记录配置加载成功日志：使用结构化日志记录关键配置信息
	slog.Info("Configuration loaded successfully",
		"path", path,
		"server_port", cfg.Server.Port,
		"database", fmt.Sprintf("%s@%s:%d/%s",
			cfg.Database.User,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Name,
		),
		"redis_nodes", cfg.Redis.ClusterNodes,
		"kafka_brokers", cfg.Kafka.Brokers,
		"kafka_topic", cfg.Kafka.Topic,
		"etcd_host", cfg.Etcd.Host,
		"log_level", cfg.Log.Level,
		"log_file_path", cfg.Log.FilePath,
		"log_max_size", cfg.Log.MaxSize,
	)
	return nil
}

// initLogger 初始化slog日志系统
// 创建双重日志处理器：同时输出到控制台和文件
// 生产环境使用JSON格式，开发环境使用文本格式
// 支持日志文件轮转，防止单个文件过大
func initLogger() error {
	// 设置日志级别：将字符串级别的日志级别转换为slog.Level类型
	var level slog.Level
	switch AppConfig.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 创建日志目录：如果目录不存在则递归创建
	logDir := AppConfig.Log.FilePath
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// 生成日志文件名：使用时间戳格式确保文件名唯一
	logFileName := generateLogFileName()
	logFilePath := filepath.Join(logDir, logFileName)

	// 创建文件日志处理器：支持日志轮转功能
	fileHandler, err := createFileHandler(logFilePath, level)
	if err != nil {
		return fmt.Errorf("failed to create file handler: %v", err)
	}

	// 创建控制台日志处理器：用于开发时的实时查看
	consoleHandler := createConsoleHandler(level)

	// 创建多路处理器：同时向控制台和文件输出日志
	multiHandler := newMultiHandler(consoleHandler, fileHandler)

	// 设置全局默认logger：所有使用slog包的日志调用都会使用这个logger
	logger := slog.New(multiHandler)
	slog.SetDefault(logger)

	// 记录日志系统初始化成功信息
	slog.Info("Logger initialized successfully",
		"level", level.String(),
		"environment", AppConfig.Environment,
		"log_file", logFilePath,
		"max_size_mb", AppConfig.Log.MaxSize,
	)
	return nil
}

// generateLogFileName 生成基于时间戳的日志文件名
// 格式：YYYYMMDD-HHMMSS.log，如：20250829-143056.log
// 这种命名方式可以方便地按时间排序和查找日志文件
func generateLogFileName() string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s.log", timestamp)
}

// createFileHandler 创建文件日志处理器
// 打开或创建日志文件，根据环境选择日志格式
// 包装为rotatingFileHandler以支持文件大小轮转
func createFileHandler(filePath string, level slog.Level) (slog.Handler, error) {
	// 打开日志文件：使用追加模式，如果文件不存在则创建
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// 根据环境选择日志格式：生产环境用JSON便于解析，开发环境用文本便于阅读
	var handler slog.Handler
	if AppConfig.Environment == "production" {
		handler = slog.NewJSONHandler(file, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(file, &slog.HandlerOptions{
			Level: level,
		})
	}

	// 包装为轮转文件处理器：监控文件大小并在需要时自动轮转
	return &rotatingFileHandler{
		handler:  handler,
		file:     file,
		filePath: filePath,
		maxSize:  AppConfig.Log.MaxSize * 1024 * 1024, // 将MB转换为字节
	}, nil
}

// createConsoleHandler 创建控制台日志处理器
// 根据运行环境选择适当的输出格式
func createConsoleHandler(level slog.Level) slog.Handler {
	if AppConfig.Environment == "production" {
		return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		return slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}
}

// multiHandler 多路日志处理器
// 实现slog.Handler接口，将日志消息同时发送到多个处理器
// 用于同时输出到控制台和文件的需求
type multiHandler struct {
	handlers []slog.Handler
}

// newMultiHandler 创建多路处理器实例
// 接收多个slog.Handler作为参数，返回一个组合处理器
func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{
		handlers: handlers,
	}
}

// Enabled 检查是否启用指定级别的日志
// 只要有一个处理器启用该级别，就返回true
func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range m.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle 处理日志记录
// 将日志记录发送到所有启用的处理器
// 如果某个处理器处理失败，记录错误但继续处理其他处理器
func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var lastErr error
	for _, handler := range m.handlers {
		if err := handler.Handle(ctx, record); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// WithAttrs 创建带有附加属性的新处理器
// 为所有子处理器添加相同的属性集
func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, handler := range m.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return newMultiHandler(handlers...)
}

// WithGroup 创建分组处理器
// 为所有子处理器创建相同的日志分组
func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, handler := range m.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return newMultiHandler(handlers...)
}

// rotatingFileHandler 支持轮转的文件日志处理器
// 监控日志文件大小，在达到限制时自动创建新文件
// 保持slog.Handler接口兼容性
type rotatingFileHandler struct {
	handler  slog.Handler
	file     *os.File
	filePath string
	maxSize  int64
}

// Enabled 委托给内部处理器的Enabled方法
func (r *rotatingFileHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return r.handler.Enabled(ctx, level)
}

// Handle 处理日志记录，在写入前检查是否需要轮转文件
func (r *rotatingFileHandler) Handle(ctx context.Context, record slog.Record) error {
	// 检查文件大小，如果需要则执行轮转
	if err := r.rotateIfNeeded(); err != nil {
		return err
	}
	return r.handler.Handle(ctx, record)
}

// WithAttrs 创建带有附加属性的新轮转处理器
func (r *rotatingFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &rotatingFileHandler{
		handler:  r.handler.WithAttrs(attrs),
		file:     r.file,
		filePath: r.filePath,
		maxSize:  r.maxSize,
	}
}

// WithGroup 创建分组轮转处理器
func (r *rotatingFileHandler) WithGroup(name string) slog.Handler {
	return &rotatingFileHandler{
		handler:  r.handler.WithGroup(name),
		file:     r.file,
		filePath: r.filePath,
		maxSize:  r.maxSize,
	}
}

// rotateIfNeeded 检查并执行日志文件轮转
// 当当前日志文件大小超过maxSize时：
// 1. 关闭当前文件
// 2. 重命名为带时间戳的备份文件
// 3. 创建新的日志文件
// 4. 更新处理器指向新文件
func (r *rotatingFileHandler) rotateIfNeeded() error {
	// 获取当前文件信息，检查文件大小
	fileInfo, err := r.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// 如果文件大小超过限制，执行轮转操作
	if fileInfo.Size() >= r.maxSize {
		// 关闭当前日志文件
		if err := r.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %v", err)
		}

		// 重命名当前文件为备份文件，添加时间戳后缀
		oldPath := r.filePath
		timestamp := time.Now().Format("20060102-150405")
		newPath := fmt.Sprintf("%s.%s", oldPath, timestamp)

		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rotate log file: %v", err)
		}

		// 创建新的日志文件，使用原始文件名
		newFile, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create new log file: %v", err)
		}

		// 更新处理器状态，指向新文件
		r.file = newFile
		// 根据原处理器类型创建新的处理器
		if r.handler != nil {
			if _, ok := r.handler.(*slog.TextHandler); ok {
				r.handler = slog.NewTextHandler(newFile, nil)
			} else if _, ok := r.handler.(*slog.JSONHandler); ok {
				r.handler = slog.NewJSONHandler(newFile, nil)
			}
		}

		// 记录轮转操作日志
		slog.Info("Log file rotated",
			"old_file", newPath,
			"new_file", oldPath,
			"max_size_mb", r.maxSize/(1024*1024),
		)
	}
	return nil
}

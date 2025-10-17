package config

import (
	"fmt"
	"log"
	"os"
	"strings"

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

// Config 聚合所有配置项
type Config struct {
	Server   ServerConfig `yaml:"server"`   // 服务器配置
	Database MysqlConfig  `yaml:"database"` // MySQL数据库配置
	Redis    RedisConfig  `yaml:"redis"`    // Redis配置
	Kafka    KafkaConfig  `yaml:"kafka"`    // Kafka配置
	Etcd     EtcdConfig   `yaml:"etcd"`     // Etcd配置
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
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

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

	if cfg.Redis.ClusterNodes == "" {
		return fmt.Errorf("redis cluster nodes are required")
	}
	nodes := cfg.Redis.GetRedisClusterNodes()
	if len(nodes) == 0 {
		return fmt.Errorf("no valid redis cluster nodes found")
	}

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

	if cfg.Etcd.Host == "" {
		return fmt.Errorf("etcd host is required")
	}
	if cfg.Etcd.DialTimeout <= 0 {
		return fmt.Errorf("etcd dial timeout must be positive")
	}

	return nil
}

// InitConfig 从指定路径加载YAML配置文件
func InitConfig(path string) error {
	// 读取配置文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析YAML配置
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// 验证配置完整性
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %v", err)
	}

	// 将解析后的配置赋值给全局变量
	AppConfig = &cfg
	log.Printf("Configuration loaded successfully from: %s", path)
	log.Printf("Server Port: %d", cfg.Server.Port)
	log.Printf("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
	log.Printf("Redis Nodes: %s", cfg.Redis.ClusterNodes)
	log.Printf("Kafka Brokers: %s, Topic: %s", cfg.Kafka.Brokers, cfg.Kafka.Topic)
	log.Printf("Etcd Host: %s", cfg.Etcd.Host)

	return nil
}

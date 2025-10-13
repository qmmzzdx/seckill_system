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

	// 将解析后的配置赋值给全局变量
	AppConfig = &cfg
	log.Printf("Configuration loaded successfully from: %s", path)
	return nil
}

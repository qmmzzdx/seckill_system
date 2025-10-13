package global

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"seckill_system/config"
	"seckill_system/model"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 全局变量定义
var (
	DBClient           *gorm.DB             // MySQL数据库客户端
	RedisClusterClient *redis.ClusterClient // Redis集群客户端
	KafkaWriter        *kafka.Writer        // Kafka生产者
	KafkaReader        *kafka.Reader        // Kafka消费者
	EtcdClient         *clientv3.Client     // Etcd客户端
	BookStockCount     = 100                // 默认书籍库存数量
)

// Etcd相关配置键常量
const (
	EtcdKeySeckillEnabled = "/seckill/config/enabled"       // 秒杀开关配置键
	EtcdKeyRateLimit      = "/seckill/config/rate_limit"    // 限流配置键
	EtcdKeyStockPreload   = "/seckill/config/stock_preload" // 库存预加载配置键
	EtcdKeyBlacklist      = "/seckill/blacklist/"           // 用户黑名单前缀
)

// InitMySQL 初始化MySQL数据库连接
func InitMySQL() {
	cfg := config.AppConfig.Database
	// 构建数据库连接字符串
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	var err error
	// 创建数据库连接
	DBClient, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 设置日志级别
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// 获取底层sql.DB对象以设置连接池参数
	sqlDB, err := DBClient.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(100)                // 最大打开连接数
	sqlDB.SetMaxIdleConns(20)                 // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(3 * time.Minute) // 连接最大生命周期

	// 初始化数据库表结构和测试数据
	if err := initDatabase(); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
}

// InitRedis 初始化Redis集群连接
func InitRedis() {
	cfg := config.AppConfig.Redis
	nodes := cfg.GetRedisClusterNodes() // 获取Redis集群节点列表

	// 创建Redis集群客户端
	RedisClusterClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        nodes,        // 集群节点地址
		Password:     cfg.Password, // 访问密码
		PoolSize:     1000,         // 连接池大小
		MinIdleConns: 10,           // 最小空闲连接数
	})

	// 测试连接是否成功
	if _, err := RedisClusterClient.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("failed to connect redis cluster: %v", err)
	}
}

// InitKafka 初始化Kafka生产者和消费者
func InitKafka() {
	cfg := config.AppConfig.Kafka
	brokers := cfg.GetKafkaBrokers() // 获取Kafka broker地址列表

	// 初始化Kafka生产者
	KafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(brokers...), // broker地址
		Topic:    cfg.Topic,             // 主题名称
		Balancer: &kafka.LeastBytes{},   // 负载均衡策略
		Async:    true,                  // 异步模式
	}

	// 初始化Kafka消费者
	KafkaReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,     // broker地址
		Topic:    cfg.Topic,   // 主题名称
		GroupID:  cfg.GroupID, // 消费者组ID
		MinBytes: 10e3,        // 最小读取字节数
		MaxBytes: 10e6,        // 最大读取字节数
	})
}

// InitEtcd 初始化Etcd客户端连接
func InitEtcd() {
	cfg := config.AppConfig.Etcd
	endpoints := cfg.GetEtcdEndpoints() // 获取Etcd服务端点

	// 创建Etcd客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,                                    // 服务端点
		DialTimeout:          time.Duration(cfg.DialTimeout) * time.Second, // 连接超时时间
		Username:             cfg.Username,                                 // 认证用户名
		Password:             cfg.Password,                                 // 认证密码
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 3 * time.Second,
		MaxCallSendMsgSize:   10 * 1024 * 1024,
		MaxCallRecvMsgSize:   10 * 1024 * 1024,
	})
	if err != nil {
		log.Fatalf("failed to connect etcd: %v", err)
	}

	// 检查Etcd服务状态
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := client.Status(ctx, endpoints[0]); err != nil {
		log.Fatalf("failed to get etcd status: %v", err)
	}

	EtcdClient = client
	log.Printf("Etcd connected successfully to: %v", endpoints)

	// 初始化Etcd中的默认配置
	initEtcdConfig()
}

// initEtcdConfig 初始化Etcd中的默认配置
func initEtcdConfig() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 定义默认配置项
	defaultConfigs := map[string]string{
		EtcdKeySeckillEnabled: "true", // 默认开启秒杀
		EtcdKeyRateLimit:      "10",   // 默认限流10次/分钟
		EtcdKeyStockPreload:   "true", // 默认开启库存预加载
	}

	// 遍历并设置默认配置
	for key, value := range defaultConfigs {
		// 检查配置是否已存在
		resp, err := EtcdClient.Get(ctx, key)
		if err != nil {
			log.Printf("Failed to check etcd key %s: %v", key, err)
			continue
		}

		// 如果配置不存在，则设置默认值
		if len(resp.Kvs) == 0 {
			_, err := EtcdClient.Put(ctx, key, value)
			if err != nil {
				log.Printf("Failed to set etcd config %s: %v", key, err)
			} else {
				log.Printf("Set default etcd config: %s=%s", key, value)
			}
		}
	}
}

// initDatabase 初始化数据库表结构和测试数据
func initDatabase() error {
	// 自动迁移数据库表
	if err := DBClient.AutoMigrate(
		&model.Goods{},
		&model.PromotionSecKill{},
		&model.SuccessKilled{},
	); err != nil {
		return fmt.Errorf("failed to auto migrate tables: %v", err)
	}

	// 插入测试数据
	return insertTestData(1000)
}

// insertTestData 向数据库插入测试数据
func insertTestData(count int) error {
	// 检查是否已有数据
	var existingCount int64
	if err := DBClient.Model(&model.Goods{}).Count(&existingCount).Error; err != nil {
		return err
	}
	if existingCount > 0 {
		return nil
	}
	// 在事务中同时插入商品和促销数据
	return DBClient.Transaction(func(tx *gorm.DB) error {
		// 生成商品数据
		goods := generateGoodsData(count)
		if err := tx.CreateInBatches(goods, count).Error; err != nil {
			return fmt.Errorf("failed to insert goods data: %v", err)
		}
		// 生成促销数据（直接使用内存中的商品数据）
		promotions := generatePromotionData(goods)
		if err := tx.CreateInBatches(promotions, count).Error; err != nil {
			return fmt.Errorf("failed to insert promotion data: %v", err)
		}
		return nil
	})
}

// generateGoodsData 生成商品测试数据
func generateGoodsData(count int) []model.Goods {
	goods := make([]model.Goods, count)
	categories := []int64{1, 2, 3, 4, 5}                                         // 商品分类ID
	bookTypes := []string{"Computer", "Literature", "Science", "History", "Art"} // 书籍类型

	// 使用随机数生成器创建随机数据
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range goods {
		originalCost := float64(r.Intn(480) + 20)     // 原始价格(20-500)
		discount := 0.6 + r.Float64()*0.35            // 折扣(0.6-0.95)
		bookType := bookTypes[r.Intn(len(bookTypes))] // 随机选择书籍类型
		serialNumber := r.Intn(1000) + 1              // 序列号

		goods[i] = model.Goods{
			GoodsId:        int64(1000 + i),                                   // 商品ID
			Title:          fmt.Sprintf("%s Book-%d", bookType, serialNumber), // 标题
			SubTitle:       fmt.Sprintf("High-quality %s book", bookType),     // 副标题
			OriginalCost:   originalCost,                                      // 原价
			CurrentPrice:   originalCost * discount,                           // 当前价格
			Discount:       discount,                                          // 折扣
			IsFreeDelivery: int32(r.Intn(2)),                                  // 是否包邮(0或1)
			CategoryId:     categories[r.Intn(len(categories))],               // 分类ID
			LastUpdateTime: time.Now(),                                        // 最后更新时间
		}
	}
	return goods
}

// generatePromotionData 生成促销测试数据，基于已生成的商品数据
func generatePromotionData(goods []model.Goods) []model.PromotionSecKill {
	promotions := make([]model.PromotionSecKill, len(goods))
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i, good := range goods {
		// 确保秒杀活动在当前时间有效
		startTime := time.Now().Add(-time.Duration(r.Intn(60)) * time.Minute) // 0-60分钟前开始
		endTime := time.Now().Add(time.Duration(r.Intn(48)+24) * time.Hour)   // 24-72小时后结束

		promotions[i] = model.PromotionSecKill{
			PsId:         int64(2000 + i),
			GoodsId:      good.GoodsId,
			PsCount:      int64(BookStockCount),
			StartTime:    startTime,
			EndTime:      endTime,
			Status:       1,
			CurrentPrice: good.CurrentPrice * 0.8,
			Version:      0,
		}
	}
	return promotions
}

// CloseMysql 关闭MySQL数据库连接
func CloseMysql() {
	if DBClient != nil {
		if sqlDB, err := DBClient.DB(); err == nil {
			sqlDB.Close()
		}
	}
}

// CloseRedis 关闭Redis集群连接
func CloseRedis() {
	if RedisClusterClient != nil {
		RedisClusterClient.Close()
	}
}

// CloseKafka 关闭Kafka生产者和消费者
func CloseKafka() {
	if KafkaWriter != nil {
		KafkaWriter.Close()
	}
	if KafkaReader != nil {
		KafkaReader.Close()
	}
}

// CloseEtcd 关闭Etcd客户端连接
func CloseEtcd() {
	if EtcdClient != nil {
		EtcdClient.Close()
	}
}

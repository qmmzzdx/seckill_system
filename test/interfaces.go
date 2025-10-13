package test

import (
	"context"
	"seckill_system/model"
	"time"

	"gorm.io/gorm"
)

// Repository interfaces for mocking
// 仓库接口定义，用于测试时的模拟实现

// GoodRepository 商品仓库接口
type GoodRepository interface {
	// FindGoodById 根据商品ID查询商品信息
	FindGoodById(goodsId int64) (model.Goods, error)
	// GetPromotionByGoodsId 根据商品ID查询促销信息
	GetPromotionByGoodsId(goodsId int64) (model.PromotionSecKill, error)
	// OccReduceOnePromotionByGoodsId 根据商品ID和版本号减少促销库存（乐观锁）
	OccReduceOnePromotionByGoodsId(goodsId int64, version int64) (int64, error)
	// AddSuccessKilled 添加秒杀成功记录
	AddSuccessKilled(tx *gorm.DB, order *model.SuccessKilled) error
	// WithTransaction 执行数据库事务
	WithTransaction(fn func(tx *gorm.DB) error) error
}

// RedisRepository Redis仓库接口
type RedisRepository interface {
	// GetGoodsStock 获取商品库存
	GetGoodsStock(goodsId int64) (int64, error)
	// DecrGoodsStock 减少商品库存
	DecrGoodsStock(goodsId int64) (int64, error)
	// IncrGoodsStock 增加商品库存
	IncrGoodsStock(goodsId int64) (int64, error)
	// SetGoodsStock 设置商品库存
	SetGoodsStock(goodsId int64, stock int64) error
	// GenerateSeckillToken 生成秒杀令牌
	GenerateSeckillToken(userId, goodsId int64) (string, error)
	// VerifySeckillToken 验证秒杀令牌
	VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error)
	// UserRateLimit 用户限流检查
	UserRateLimit(userId int64, limit int64, duration time.Duration) (bool, error)
}

// KafkaRepository Kafka消息仓库接口
type KafkaRepository interface {
	// SendOrderMessage 发送订单消息
	SendOrderMessage(ctx context.Context, order *model.OrderMessage) error
	// SendPaymentMessage 发送支付消息
	SendPaymentMessage(ctx context.Context, orderId string, status int32) error
}

// ETCDRepository ETCD配置仓库接口
type ETCDRepository interface {
	// GetSeckillEnabled 获取秒杀开关状态
	GetSeckillEnabled(ctx context.Context) (bool, error)
	// GetDistributedLock 获取分布式锁
	GetDistributedLock(ctx context.Context, key string, ttl int) (bool, error)
	// ReleaseDistributedLock 释放分布式锁
	ReleaseDistributedLock(ctx context.Context, key string) error
	// IsInBlacklist 检查用户是否在黑名单中
	IsInBlacklist(ctx context.Context, userId int64) (bool, error)
	// GetRateLimitConfig 获取限流配置
	GetRateLimitConfig(ctx context.Context) (int64, error)
}

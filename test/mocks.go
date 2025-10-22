package test

import (
	"context"
	"errors"
	"seckill_system/model"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// MockGoodRepository 商品仓库的模拟实现
type MockGoodRepository struct {
	GoodsData      map[int64]model.Goods            // 商品数据存储
	PromotionData  map[int64]model.PromotionSecKill // 促销数据存储
	SuccessKilled  []model.SuccessKilled            // 秒杀成功记录
	ShouldError    bool                             // 是否模拟错误
	ReduceStockErr error                            // 减少库存错误
}

// NewMockGoodRepository 创建模拟商品仓库实例
func NewMockGoodRepository() *MockGoodRepository {
	return &MockGoodRepository{
		GoodsData:     make(map[int64]model.Goods),
		PromotionData: make(map[int64]model.PromotionSecKill),
	}
}

// FindGoodById 根据商品ID查询商品信息
func (m *MockGoodRepository) FindGoodById(goodsId int64) (model.Goods, error) {
	if m.ShouldError {
		return model.Goods{}, errors.New("mock error")
	}
	good, exists := m.GoodsData[goodsId]
	if !exists {
		return model.Goods{}, gorm.ErrRecordNotFound
	}
	return good, nil
}

// GetPromotionByGoodsId 根据商品ID查询促销信息
func (m *MockGoodRepository) GetPromotionByGoodsId(goodsId int64) (model.PromotionSecKill, error) {
	if m.ShouldError {
		return model.PromotionSecKill{}, errors.New("mock error")
	}
	promotion, exists := m.PromotionData[goodsId]
	if !exists {
		return model.PromotionSecKill{}, gorm.ErrRecordNotFound
	}
	return promotion, nil
}

// OccReduceOnePromotionByGoodsId 使用乐观锁减少促销库存
func (m *MockGoodRepository) OccReduceOnePromotionByGoodsId(goodsId int64, version int64) (int64, error) {
	if m.ReduceStockErr != nil {
		return 0, m.ReduceStockErr
	}

	promotion, exists := m.PromotionData[goodsId]
	if !exists {
		return 0, gorm.ErrRecordNotFound
	}

	if promotion.Version != version {
		return 0, nil // 乐观锁冲突，版本号不匹配
	}

	if promotion.PsCount <= 0 {
		return 0, nil // 库存不足
	}

	// 模拟更新库存和版本号
	promotion.PsCount--
	promotion.Version++
	m.PromotionData[goodsId] = promotion

	return 1, nil
}

// AddSuccessKilled 添加秒杀成功记录
func (m *MockGoodRepository) AddSuccessKilled(tx *gorm.DB, order *model.SuccessKilled) error {
	if m.ShouldError {
		return errors.New("mock error")
	}
	m.SuccessKilled = append(m.SuccessKilled, *order)
	return nil
}

// WithTransaction 执行数据库事务
func (m *MockGoodRepository) WithTransaction(fn func(tx *gorm.DB) error) error {
	return fn(nil) // 简化实现，实际应该模拟事务
}

// MockRedisRepository Redis仓库的模拟实现
type MockRedisRepository struct {
	StockData     map[int64]int64                    // 商品库存数据
	Tokens        map[string]model.RedisSeckillToken // 秒杀令牌存储
	UserRateCount map[int64]int64                    // 用户请求计数
	ShouldError   bool                               // 是否模拟错误
	LastRateReset time.Time                          // 上次限流重置时间
}

// NewMockRedisRepository 创建模拟Redis仓库实例
func NewMockRedisRepository() *MockRedisRepository {
	return &MockRedisRepository{
		StockData:     make(map[int64]int64),
		Tokens:        make(map[string]model.RedisSeckillToken),
		UserRateCount: make(map[int64]int64),
	}
}

// GetGoodsStock 获取商品库存
func (m *MockRedisRepository) GetGoodsStock(goodsId int64) (int64, error) {
	if m.ShouldError {
		return 0, errors.New("mock error")
	}
	return m.StockData[goodsId], nil
}

// DecrGoodsStock 减少商品库存
func (m *MockRedisRepository) DecrGoodsStock(goodsId int64) (int64, error) {
	if m.ShouldError {
		return 0, errors.New("mock error")
	}
	m.StockData[goodsId]--
	return m.StockData[goodsId], nil
}

// IncrGoodsStock 增加商品库存
func (m *MockRedisRepository) IncrGoodsStock(goodsId int64) (int64, error) {
	if m.ShouldError {
		return 0, errors.New("mock error")
	}
	m.StockData[goodsId]++
	return m.StockData[goodsId], nil
}

// SetGoodsStock 设置商品库存
func (m *MockRedisRepository) SetGoodsStock(goodsId int64, stock int64) error {
	if m.ShouldError {
		return errors.New("mock error")
	}
	m.StockData[goodsId] = stock
	return nil
}

// GenerateSeckillToken 生成秒杀令牌
func (m *MockRedisRepository) GenerateSeckillToken(userId, goodsId int64) (string, error) {
	if m.ShouldError {
		return "", errors.New("mock error")
	}
	token := &model.RedisSeckillToken{
		TokenId:   "mock-token",
		UserId:    userId,
		GoodsId:   goodsId,
		ExpireAt:  time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	}
	m.Tokens["mock-token"] = *token
	return "mock-token", nil
}

// VerifySeckillToken 验证秒杀令牌
func (m *MockRedisRepository) VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error) {
	if m.ShouldError {
		return false, errors.New("mock error")
	}
	token, exists := m.Tokens[tokenId]
	if !exists {
		return false, nil // 令牌不存在
	}
	if token.UserId != userId || token.GoodsId != goodsId {
		return false, nil // 用户或商品不匹配
	}
	if time.Now().After(token.ExpireAt) {
		delete(m.Tokens, tokenId) // 删除过期令牌
		return false, nil
	}
	delete(m.Tokens, tokenId) // 一次性使用，验证后删除
	return true, nil
}

// UserRateLimit 用户限流检查
func (m *MockRedisRepository) UserRateLimit(userId int64, limit int64, duration time.Duration) (bool, error) {
	if m.ShouldError {
		return false, errors.New("mock error")
	}

	// 简单的限流实现：检查时间窗口是否过期
	if time.Since(m.LastRateReset) > duration {
		m.UserRateCount = make(map[int64]int64) // 重置计数
		m.LastRateReset = time.Now()
	}

	m.UserRateCount[userId]++                    // 增加用户请求计数
	return m.UserRateCount[userId] <= limit, nil // 检查是否超过限制
}

// MockKafkaRepository Kafka仓库的模拟实现
type MockKafkaRepository struct {
	Messages       []any // 消息存储
	ShouldError    bool          // 是否模拟错误
	SendOrderErr   error         // 发送订单消息错误
	SendPaymentErr error         // 发送支付消息错误
}

// NewMockKafkaRepository 创建模拟Kafka仓库实例
func NewMockKafkaRepository() *MockKafkaRepository {
	return &MockKafkaRepository{
		Messages: make([]any, 0),
	}
}

// SendOrderMessage 发送订单消息
func (m *MockKafkaRepository) SendOrderMessage(ctx context.Context, order *model.OrderMessage) error {
	if m.ShouldError || m.SendOrderErr != nil {
		return errors.New("mock kafka error")
	}
	m.Messages = append(m.Messages, order)
	return nil
}

// SendPaymentMessage 发送支付消息
func (m *MockKafkaRepository) SendPaymentMessage(ctx context.Context, orderId string, status int32) error {
	if m.ShouldError || m.SendPaymentErr != nil {
		return errors.New("mock kafka error")
	}
	m.Messages = append(m.Messages, map[string]any{
		"order_id": orderId,
		"status":   status,
	})
	return nil
}

// MockETCDRepository ETCD仓库的模拟实现
type MockETCDRepository struct {
	Configs     map[string]string // 配置数据
	Blacklist   map[int64]bool    // 黑名单数据
	Locks       map[string]bool   // 分布式锁状态
	ShouldError bool              // 是否模拟错误
}

// NewMockETCDRepository 创建模拟ETCD仓库实例
func NewMockETCDRepository() *MockETCDRepository {
	return &MockETCDRepository{
		Configs: map[string]string{
			"/seckill/config/enabled":    "true", // 默认秒杀开启
			"/seckill/config/rate_limit": "10",   // 默认限流10
		},
		Blacklist: make(map[int64]bool),
		Locks:     make(map[string]bool),
	}
}

// GetSeckillEnabled 获取秒杀开关状态
func (m *MockETCDRepository) GetSeckillEnabled(ctx context.Context) (bool, error) {
	if m.ShouldError {
		return false, errors.New("mock error")
	}
	return m.Configs["/seckill/config/enabled"] == "true", nil
}

// GetDistributedLock 获取分布式锁
func (m *MockETCDRepository) GetDistributedLock(ctx context.Context, key string, ttl int) (bool, error) {
	if m.ShouldError {
		return false, errors.New("mock error")
	}
	if m.Locks[key] {
		return false, nil // 锁已被占用，获取失败
	}
	m.Locks[key] = true // 获取锁成功
	return true, nil
}

// ReleaseDistributedLock 释放分布式锁
func (m *MockETCDRepository) ReleaseDistributedLock(ctx context.Context, key string) error {
	if m.ShouldError {
		return errors.New("mock error")
	}
	delete(m.Locks, key) // 释放锁
	return nil
}

// IsInBlacklist 检查用户是否在黑名单中
func (m *MockETCDRepository) IsInBlacklist(ctx context.Context, userId int64) (bool, error) {
	if m.ShouldError {
		return false, errors.New("mock error")
	}
	return m.Blacklist[userId], nil
}

// GetRateLimitConfig 获取限流配置
func (m *MockETCDRepository) GetRateLimitConfig(ctx context.Context) (int64, error) {
	if m.ShouldError {
		return 0, errors.New("mock error")
	}

	limitStr := m.Configs["/seckill/config/rate_limit"]
	if limitStr == "" {
		return 10, nil // 默认限流值
	}

	// 正确解析字符串为int64
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		return 10, nil // 解析失败时返回默认值
	}
	return limit, nil
}

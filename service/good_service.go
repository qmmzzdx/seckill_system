package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"seckill_system/global"
	"seckill_system/handler"
	"seckill_system/model"
	"seckill_system/repository"
	"sync"
	"time"
)

// 单例模式相关变量
var (
	goodServiceInstance *GoodService
	goodServiceOnce     sync.Once
)

// GoodService 秒杀商品服务，封装核心业务逻辑
type GoodService struct {
	GoodDB         *repository.GoodRepository  // 商品数据库操作
	RedisRepo      *repository.RedisRepository // Redis操作
	KafkaRepo      *repository.KafkaRepository // Kafka消息队列操作
	EtcdRepo       *repository.ETCDRepository  // ETCD配置中心操作
	SeckillHandler *handler.SeckillHandler     // 秒杀处理器
}

// NewGoodService 创建商品服务实例
func NewGoodService() *GoodService {
	service := &GoodService{
		GoodDB:         repository.NewGoodRepository(),
		RedisRepo:      repository.NewRedisRepository(),
		KafkaRepo:      repository.NewKafkaRepository(),
		EtcdRepo:       repository.NewETCDRepository(),
		SeckillHandler: handler.NewSeckillHandler(),
	}

	service.StartOrderConsumer()   // 启动订单消息消费者
	service.StartPaymentConsumer() // 启动支付消息消费者
	service.StartConfigWatcher()   // 启动配置变更监听

	slog.Info("GoodService initialized successfully")
	return service
}

// GetGoodService 获取商品服务单例
func GetGoodService() *GoodService {
	goodServiceOnce.Do(func() {
		goodServiceInstance = NewGoodService()
	})
	return goodServiceInstance
}

// GenerateUserToken 生成用户令牌(JWT)
func (gs *GoodService) GenerateUserToken(userId int64) (string, error) {
	token, err := gs.RedisRepo.GenerateUserToken(userId)
	if err != nil {
		slog.Error("Failed to generate user token",
			"user_id", userId,
			"error", err,
		)
		return "", err
	}

	slog.Info("User token generated",
		"user_id", userId,
		"token", token,
	)
	return token, nil
}

// VerifyUserToken 验证用户令牌
func (gs *GoodService) VerifyUserToken(token string) (int64, error) {
	userId, err := gs.RedisRepo.VerifyUserToken(token)
	if err != nil {
		slog.Warn("User token verification failed",
			"token", token,
			"error", err,
		)
		return 0, err
	}

	slog.Info("User token verified",
		"user_id", userId,
		"token", token,
	)
	return userId, nil
}

// GenerateSeckillToken 生成秒杀令牌(包含多重校验)
func (gs *GoodService) GenerateSeckillToken(userId, goodsId int64) (string, error) {
	// 用户级锁，防止同一用户重复获取令牌
	userLockKey := fmt.Sprintf("user_token_lock_%d_%d", userId, goodsId)

	// 使用带超时的context
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer lockCancel()

	locked, err := gs.EtcdRepo.GetDistributedLock(lockCtx, userLockKey, 10)
	if err != nil || !locked {
		slog.Warn("Failed to acquire user token lock",
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		return "", fmt.Errorf("please don't repeat request: %v", err)
	}
	defer func() {
		// 使用新的context释放锁，避免使用已取消的context
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer releaseCancel()
		if releaseErr := gs.EtcdRepo.ReleaseDistributedLock(releaseCtx, userLockKey); releaseErr != nil {
			slog.Warn("Failed to release user token lock",
				"user_id", userId,
				"goods_id", goodsId,
				"error", releaseErr,
			)
		}
	}()

	// 检查秒杀系统是否开启
	enabled, err := gs.EtcdRepo.GetSeckillEnabled(context.Background())
	if err != nil {
		slog.Error("Failed to check seckill enabled status",
			"error", err,
		)
		return "", fmt.Errorf("check seckill enabled failed: %v", err)
	}
	if !enabled {
		slog.Warn("Seckill system is disabled",
			"user_id", userId,
			"goods_id", goodsId,
		)
		return "", errors.New("seckill system is temporarily disabled")
	}

	// 检查用户是否在黑名单
	inBlacklist, err := gs.EtcdRepo.IsInBlacklist(context.Background(), userId)
	if err != nil {
		slog.Error("Failed to check blacklist",
			"user_id", userId,
			"error", err,
		)
		return "", fmt.Errorf("check blacklist failed: %v", err)
	}
	if inBlacklist {
		slog.Warn("User in blacklist attempted to get seckill token",
			"user_id", userId,
			"goods_id", goodsId,
		)
		return "", errors.New("user is in blacklist")
	}

	// 检查商品是否存在
	_, err = gs.FindGoodById(goodsId)
	if err != nil {
		slog.Warn("Goods not found for seckill token",
			"goods_id", goodsId,
			"error", err,
		)
		return "", fmt.Errorf("find goods failed: %v", err)
	}

	// 检查秒杀活动时间
	promotion, err := gs.GetPromotionByGoodsId(goodsId)
	if err != nil {
		slog.Warn("Promotion not found for seckill token",
			"goods_id", goodsId,
			"error", err,
		)
		return "", fmt.Errorf("find promotion failed: %v", err)
	}

	now := time.Now()
	slog.Info("Promotion time check",
		"goods_id", goodsId,
		"now", now,
		"start_time", promotion.StartTime,
		"end_time", promotion.EndTime,
		"before_start", now.Before(promotion.StartTime),
		"after_end", now.After(promotion.EndTime),
	)

	if now.Before(promotion.StartTime) || now.After(promotion.EndTime) {
		slog.Warn("Seckill activity not available at current time",
			"goods_id", goodsId,
			"now", now,
			"start_time", promotion.StartTime,
			"end_time", promotion.EndTime,
		)
		return "", errors.New("seckill activity is not available")
	}

	// 检查库存
	stock, err := gs.SeckillHandler.CheckStock(context.Background(), goodsId)
	if err != nil || stock <= 0 {
		slog.Warn("Insufficient stock for seckill token",
			"goods_id", goodsId,
			"stock", stock,
			"error", err,
		)
		return "", errors.New("goods sold out")
	}

	// 限流检查
	rateLimit, err := gs.EtcdRepo.GetRateLimitConfig(context.Background())
	if err != nil {
		rateLimit = 10 // 默认限流值
		slog.Warn("Failed to get rate limit config, using default",
			"default_limit", rateLimit,
			"error", err,
		)
	}

	allowed, err := gs.RedisRepo.UserRateLimit(userId, rateLimit, time.Minute)
	if err != nil {
		slog.Error("Rate limit check failed",
			"user_id", userId,
			"error", err,
		)
		return "", fmt.Errorf("check user rate limit failed: %v", err)
	}
	if !allowed {
		slog.Warn("User rate limit exceeded",
			"user_id", userId,
			"limit", rateLimit,
		)
		return "", errors.New("too many requests")
	}

	// 生成秒杀令牌
	tokenId, err := gs.RedisRepo.GenerateSeckillToken(userId, goodsId)
	if err != nil {
		slog.Error("Failed to generate seckill token",
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		return "", err
	}

	slog.Info("Seckill token generated successfully",
		"user_id", userId,
		"goods_id", goodsId,
		"token_id_prefix", tokenId[:8],
	)
	return tokenId, nil
}

// StartConfigWatcher 启动ETCD配置监听
func (gs *GoodService) StartConfigWatcher() {
	go func() {
		slog.Info("Starting etcd config watcher...")
		// 监听秒杀配置变更
		gs.EtcdRepo.WatchSeckillConfig(context.Background(), func(key, value string) {
			slog.Info("ETCD config changed",
				"key", key,
				"value", value,
			)

			// 根据不同的配置键处理变更
			switch key {
			case global.EtcdKeySeckillEnabled:
				if value == "false" {
					slog.Warn("Seckill system has been disabled via etcd config")
				} else {
					slog.Info("Seckill system has been enabled via etcd config")
				}
			case global.EtcdKeyRateLimit:
				slog.Info("Rate limit config changed", "new_value", value)
			case global.EtcdKeyStockPreload:
				slog.Info("Stock preload config changed", "new_value", value)
			}
		})
	}()
}

// SetSeckillEnabled 设置秒杀开关状态
func (gs *GoodService) SetSeckillEnabled(enabled bool) error {
	err := gs.EtcdRepo.SetSeckillEnabled(context.Background(), enabled)
	if err != nil {
		slog.Error("Failed to set seckill enabled",
			"enabled", enabled,
			"error", err,
		)
		return err
	}

	slog.Info("Seckill enabled status updated",
		"enabled", enabled,
	)
	return nil
}

// SetRateLimit 设置限流值
func (gs *GoodService) SetRateLimit(limit int64) error {
	err := gs.EtcdRepo.SetRateLimitConfig(context.Background(), limit)
	if err != nil {
		slog.Error("Failed to set rate limit",
			"limit", limit,
			"error", err,
		)
		return err
	}

	slog.Info("Rate limit updated",
		"limit", limit,
	)
	return nil
}

// AddToBlacklist 添加用户到黑名单
func (gs *GoodService) AddToBlacklist(userId int64, reason string, duration time.Duration) error {
	err := gs.EtcdRepo.AddToBlacklist(context.Background(), userId, reason, duration)
	if err != nil {
		slog.Error("Failed to add user to blacklist",
			"user_id", userId,
			"reason", reason,
			"duration", duration,
			"error", err,
		)
		return err
	}

	slog.Info("User added to blacklist",
		"user_id", userId,
		"reason", reason,
		"duration", duration,
	)
	return nil
}

// RemoveFromBlacklist 从黑名单移除用户
func (gs *GoodService) RemoveFromBlacklist(userId int64) error {
	err := gs.EtcdRepo.RemoveFromBlacklist(context.Background(), userId)
	if err != nil {
		slog.Error("Failed to remove user from blacklist",
			"user_id", userId,
			"error", err,
		)
		return err
	}

	slog.Info("User removed from blacklist",
		"user_id", userId,
	)
	return nil
}

// GetBlacklist 获取黑名单列表
func (gs *GoodService) GetBlacklist() ([]map[string]any, error) {
	blacklist, err := gs.EtcdRepo.GetBlacklist(context.Background())
	if err != nil {
		slog.Error("Failed to get blacklist",
			"error", err,
		)
		return nil, err
	}

	slog.Info("Blacklist retrieved",
		"count", len(blacklist),
	)
	return blacklist, nil
}

// VerifySeckillToken 验证秒杀令牌
func (gs *GoodService) VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error) {
	valid, err := gs.RedisRepo.VerifySeckillToken(tokenId, userId, goodsId)
	if err != nil {
		slog.Warn("Seckill token verification failed",
			"token_id_prefix", tokenId[:8],
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		return false, err
	}

	if valid {
		slog.Info("Seckill token verified successfully",
			"token_id_prefix", tokenId[:8],
			"user_id", userId,
			"goods_id", goodsId,
		)
	} else {
		slog.Warn("Seckill token invalid",
			"token_id_prefix", tokenId[:8],
			"user_id", userId,
			"goods_id", goodsId,
		)
	}
	return valid, nil
}

// FindGoodById 根据ID查询商品
func (gs *GoodService) FindGoodById(goodsId int64) (model.Goods, error) {
	good, err := gs.GoodDB.FindGoodById(goodsId)
	if err != nil {
		slog.Warn("Good not found",
			"goods_id", goodsId,
			"error", err,
		)
		return good, err
	}

	slog.Info("Good found",
		"goods_id", goodsId,
		"title", good.Title,
	)
	return good, nil
}

// GetPromotionByGoodsId 获取商品秒杀活动信息
func (gs *GoodService) GetPromotionByGoodsId(goodsId int64) (model.PromotionSecKill, error) {
	promotion, err := gs.GoodDB.GetPromotionByGoodsId(goodsId)
	if err != nil {
		slog.Warn("Promotion not found",
			"goods_id", goodsId,
			"error", err,
		)
		return promotion, err
	}

	slog.Info("Promotion found",
		"goods_id", goodsId,
		"ps_count", promotion.PsCount,
		"current_price", promotion.CurrentPrice,
	)
	return promotion, nil
}

// PreloadGoodsStock 预加载商品库存到Redis
func (gs *GoodService) PreloadGoodsStock(goodsId int64) error {
	// 获取ETCD分布式锁，防止并发预加载
	lockKey := fmt.Sprintf("preload_lock_%d", goodsId)
	locked, err := gs.EtcdRepo.GetDistributedLock(context.Background(), lockKey, 30) // 30秒超时
	if err != nil || !locked {
		slog.Warn("Failed to acquire preload lock",
			"goods_id", goodsId,
			"error", err,
		)
		return fmt.Errorf("failed to acquire preload lock for goods %d", goodsId)
	}
	defer gs.EtcdRepo.ReleaseDistributedLock(context.Background(), lockKey)

	promotion, err := gs.GetPromotionByGoodsId(goodsId)
	if err != nil {
		slog.Error("Failed to get promotion for preload",
			"goods_id", goodsId,
			"error", err,
		)
		return err
	}

	err = gs.RedisRepo.SetGoodsStock(goodsId, promotion.PsCount)
	if err != nil {
		slog.Error("Failed to preload goods stock to Redis",
			"goods_id", goodsId,
			"stock", promotion.PsCount,
			"error", err,
		)
		return err
	}

	slog.Info("Goods stock preloaded to Redis",
		"goods_id", goodsId,
		"stock", promotion.PsCount,
	)
	return nil
}

// SeckillWithToken 使用令牌进行秒杀
func (gs *GoodService) SeckillWithToken(userId, goodsId int64, tokenId string) (string, error) {
	// 验证令牌有效性
	valid, err := gs.VerifySeckillToken(tokenId, userId, goodsId)
	if err != nil || !valid {
		slog.Warn("Invalid seckill token",
			"token_id_prefix", tokenId[:8],
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		return "", fmt.Errorf("invalid seckill token: %v", err)
	}

	// 改进分布式锁机制，避免死锁和锁竞争问题
	lockKey := fmt.Sprintf("seckill_user_%d", userId)

	// 使用独立的context获取锁
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer lockCancel()

	locked, err := gs.EtcdRepo.GetDistributedLock(lockCtx, lockKey, 10) // 延长TTL到10秒
	if err != nil {
		slog.Error("Failed to acquire distributed lock for seckill",
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		return "", fmt.Errorf("system busy, failed to acquire lock: %v", err)
	}
	if !locked {
		slog.Warn("Distributed lock acquisition failed for seckill",
			"user_id", userId,
			"goods_id", goodsId,
		)
		return "", errors.New("system busy, please try again")
	}

	// 使用新的context执行业务逻辑，避免锁过期影响业务
	businessCtx := context.Background()
	defer func() {
		// 使用新的context释放锁
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer releaseCancel()
		if releaseErr := gs.EtcdRepo.ReleaseDistributedLock(releaseCtx, lockKey); releaseErr != nil {
			slog.Warn("Failed to release distributed lock after seckill",
				"user_id", userId,
				"goods_id", goodsId,
				"error", releaseErr,
			)
		}
	}()

	orderId, err := gs.SeckillHandler.CreateOrder(businessCtx, userId, goodsId)
	if err != nil {
		slog.Error("Seckill failed",
			"user_id", userId,
			"goods_id", goodsId,
			"token_id_prefix", tokenId[:8],
			"error", err,
		)
		return "", fmt.Errorf("seckill failed: %v", err)
	}

	slog.Info("Seckill successful",
		"user_id", userId,
		"goods_id", goodsId,
		"order_id", orderId,
		"token_id_prefix", tokenId[:8],
	)
	return orderId, nil
}

// SimulatePayment 模拟支付
func (gs *GoodService) SimulatePayment(orderId string, success bool) error {
	err := gs.SeckillHandler.SimulatePayment(context.Background(), orderId, success)
	if err != nil {
		slog.Error("Payment simulation failed",
			"order_id", orderId,
			"success", success,
			"error", err,
		)
		return err
	}

	slog.Info("Payment simulation completed",
		"order_id", orderId,
		"success", success,
	)
	return nil
}

// StartOrderConsumer 启动订单消息消费者
func (gs *GoodService) StartOrderConsumer() {
	go func() {
		slog.Info("Starting order message consumer...")
		// 消费订单消息
		err := gs.KafkaRepo.ConsumeOrderMessages(context.Background(), func(order model.OrderMessage) error {
			slog.Info("Processing order message from Kafka",
				"order_id", order.OrderId,
				"user_id", order.UserId,
				"goods_id", order.GoodsId,
				"status", order.Status,
				"price", order.Price,
			)

			// 根据订单状态处理
			switch order.Status {
			case model.OrderStatusCreated:
				// 订单创建成功处理
				slog.Info("Order created, triggering follow-up actions",
					"order_id", order.OrderId,
				)

			case model.OrderStatusPaid:
				// 支付成功处理
				slog.Info("Order paid, updating order status",
					"order_id", order.OrderId,
				)

			case model.OrderStatusPaymentFailed:
				// 支付失败处理
				slog.Warn("Order payment failed, need to restore stock",
					"order_id", order.OrderId,
				)
			}

			return nil
		})
		if err != nil {
			slog.Error("Order consumer failed",
				"error", err,
			)
		}
	}()
}

// StartPaymentConsumer 启动支付消息消费者
func (gs *GoodService) StartPaymentConsumer() {
	go func() {
		slog.Info("Starting payment message consumer...")
		// 消费支付消息
		err := gs.KafkaRepo.ConsumePaymentMessages(context.Background(), func(orderId string, status int32) error {
			slog.Info("Processing payment message from Kafka",
				"order_id", orderId,
				"status", status,
			)

			// 根据支付状态处理
			switch status {
			case model.OrderStatusPaid:
				// 支付成功处理
				slog.Info("Payment successful",
					"order_id", orderId,
				)

			case model.OrderStatusPaymentFailed:
				// 支付失败处理
				slog.Warn("Payment failed, restoring stock",
					"order_id", orderId,
				)
			}

			return nil
		})
		if err != nil {
			slog.Error("Payment consumer failed",
				"error", err,
			)
		}
	}()
}

// ResetDataBase 重置数据库
func (gs *GoodService) ResetDataBase(goodsId int) error {
	err := gs.GoodDB.ResetDataBase(goodsId)
	if err != nil {
		slog.Error("Failed to reset database",
			"goods_id", goodsId,
			"error", err,
		)
		return err
	}

	slog.Info("Database reset successfully",
		"goods_id", goodsId,
	)
	return nil
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	return gs.RedisRepo.GenerateUserToken(userId)
}

// VerifyUserToken 验证用户令牌
func (gs *GoodService) VerifyUserToken(token string) (int64, error) {
	return gs.RedisRepo.VerifyUserToken(token)
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
		return "", fmt.Errorf("please don't repeat request: %v", err)
	}
	defer func() {
		// 使用新的context释放锁，避免使用已取消的context
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer releaseCancel()
		if releaseErr := gs.EtcdRepo.ReleaseDistributedLock(releaseCtx, userLockKey); releaseErr != nil {
			log.Printf("Failed to release user token lock: %v", releaseErr)
		}
	}()

	// 检查秒杀系统是否开启
	enabled, err := gs.EtcdRepo.GetSeckillEnabled(context.Background())
	if err != nil {
		return "", fmt.Errorf("check seckill enabled failed: %v", err)
	}
	if !enabled {
		return "", errors.New("seckill system is temporarily disabled")
	}

	// 检查用户是否在黑名单
	inBlacklist, err := gs.EtcdRepo.IsInBlacklist(context.Background(), userId)
	if err != nil {
		return "", fmt.Errorf("check blacklist failed: %v", err)
	}
	if inBlacklist {
		return "", errors.New("user is in blacklist")
	}

	// 检查商品是否存在
	_, err = gs.FindGoodById(goodsId)
	if err != nil {
		return "", fmt.Errorf("find goods failed: %v", err)
	}

	// 检查秒杀活动时间
	promotion, err := gs.GetPromotionByGoodsId(goodsId)
	if err != nil {
		return "", fmt.Errorf("find promotion failed: %v", err)
	}

	now := time.Now()
	log.Printf("Promotion time check - Now: %v, Start: %v, End: %v",
		now, promotion.StartTime, promotion.EndTime)
	log.Printf("Time comparison - Before start: %v, After end: %v",
		now.Before(promotion.StartTime), now.After(promotion.EndTime))

	if now.Before(promotion.StartTime) || now.After(promotion.EndTime) {
		return "", errors.New("seckill activity is not available")
	}

	// 检查库存
	stock, err := gs.SeckillHandler.CheckStock(context.Background(), goodsId)
	if err != nil || stock <= 0 {
		return "", errors.New("goods sold out")
	}

	// 限流检查
	rateLimit, err := gs.EtcdRepo.GetRateLimitConfig(context.Background())
	if err != nil {
		rateLimit = 10 // 默认限流值
	}

	allowed, err := gs.RedisRepo.UserRateLimit(userId, rateLimit, time.Minute)
	if err != nil {
		return "", fmt.Errorf("check user rate limit failed: %v", err)
	}
	if !allowed {
		return "", errors.New("too many requests")
	}

	// 生成秒杀令牌
	return gs.RedisRepo.GenerateSeckillToken(userId, goodsId)
}

// StartConfigWatcher 启动ETCD配置监听
func (gs *GoodService) StartConfigWatcher() {
	go func() {
		log.Println("Starting etcd config watcher...")
		// 监听秒杀配置变更
		gs.EtcdRepo.WatchSeckillConfig(context.Background(), func(key, value string) {
			log.Printf("Config changed - Key: %s, Value: %s", key, value)

			// 根据不同的配置键处理变更
			switch key {
			case global.EtcdKeySeckillEnabled:
				if value == "false" {
					log.Println("Seckill system has been disabled via etcd config")
				} else {
					log.Println("Seckill system has been enabled via etcd config")
				}
			case global.EtcdKeyRateLimit:
				log.Printf("Rate limit config changed to: %s", value)
			}
		})
	}()
}

// SetSeckillEnabled 设置秒杀开关状态
func (gs *GoodService) SetSeckillEnabled(enabled bool) error {
	return gs.EtcdRepo.SetSeckillEnabled(context.Background(), enabled)
}

// SetRateLimit 设置限流值
func (gs *GoodService) SetRateLimit(limit int64) error {
	return gs.EtcdRepo.SetRateLimitConfig(context.Background(), limit)
}

// AddToBlacklist 添加用户到黑名单
func (gs *GoodService) AddToBlacklist(userId int64, reason string, duration time.Duration) error {
	return gs.EtcdRepo.AddToBlacklist(context.Background(), userId, reason, duration)
}

// RemoveFromBlacklist 从黑名单移除用户
func (gs *GoodService) RemoveFromBlacklist(userId int64) error {
	return gs.EtcdRepo.RemoveFromBlacklist(context.Background(), userId)
}

// GetBlacklist 获取黑名单列表
func (gs *GoodService) GetBlacklist() ([]map[string]any, error) {
	return gs.EtcdRepo.GetBlacklist(context.Background())
}

// VerifySeckillToken 验证秒杀令牌
func (gs *GoodService) VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error) {
	return gs.RedisRepo.VerifySeckillToken(tokenId, userId, goodsId)
}

// FindGoodById 根据ID查询商品
func (gs *GoodService) FindGoodById(goodsId int64) (model.Goods, error) {
	return gs.GoodDB.FindGoodById(goodsId)
}

// GetPromotionByGoodsId 获取商品秒杀活动信息
func (gs *GoodService) GetPromotionByGoodsId(goodsId int64) (model.PromotionSecKill, error) {
	return gs.GoodDB.GetPromotionByGoodsId(goodsId)
}

// PreloadGoodsStock 预加载商品库存到Redis
func (gs *GoodService) PreloadGoodsStock(goodsId int64) error {
	// 获取ETCD分布式锁，防止并发预加载
	lockKey := fmt.Sprintf("preload_lock_%d", goodsId)
	locked, err := gs.EtcdRepo.GetDistributedLock(context.Background(), lockKey, 30) // 30秒超时
	if err != nil || !locked {
		return fmt.Errorf("failed to acquire preload lock for goods %d", goodsId)
	}
	defer gs.EtcdRepo.ReleaseDistributedLock(context.Background(), lockKey)

	promotion, err := gs.GetPromotionByGoodsId(goodsId)
	if err != nil {
		return err
	}
	return gs.RedisRepo.SetGoodsStock(goodsId, promotion.PsCount)
}

// SeckillWithToken 使用令牌进行秒杀
func (gs *GoodService) SeckillWithToken(userId, goodsId int64, tokenId string) (string, error) {
	// 验证令牌有效性
	valid, err := gs.VerifySeckillToken(tokenId, userId, goodsId)
	if err != nil || !valid {
		return "", fmt.Errorf("invalid seckill token: %v", err)
	}

	// 改进分布式锁机制，避免死锁和锁竞争问题
	lockKey := fmt.Sprintf("seckill_user_%d", userId)

	// 使用独立的context获取锁
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer lockCancel()

	locked, err := gs.EtcdRepo.GetDistributedLock(lockCtx, lockKey, 10) // 延长TTL到10秒
	if err != nil {
		return "", fmt.Errorf("system busy, failed to acquire lock: %v", err)
	}
	if !locked {
		return "", errors.New("system busy, please try again")
	}

	// 使用新的context执行业务逻辑，避免锁过期影响业务
	businessCtx := context.Background()
	defer func() {
		// 使用新的context释放锁
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer releaseCancel()
		if releaseErr := gs.EtcdRepo.ReleaseDistributedLock(releaseCtx, lockKey); releaseErr != nil {
			log.Printf("Warning: Failed to release distributed lock %s: %v", lockKey, releaseErr)
		}
	}()

	orderId, err := gs.SeckillHandler.CreateOrder(businessCtx, userId, goodsId)
	if err != nil {
		return "", fmt.Errorf("seckill failed: %v", err)
	}
	return orderId, nil
}

// SimulatePayment 模拟支付
func (gs *GoodService) SimulatePayment(orderId string, success bool) error {
	return gs.SeckillHandler.SimulatePayment(context.Background(), orderId, success)
}

// StartOrderConsumer 启动订单消息消费者
func (gs *GoodService) StartOrderConsumer() {
	go func() {
		log.Println("Starting order message consumer...")
		// 消费订单消息
		err := gs.KafkaRepo.ConsumeOrderMessages(context.Background(), func(order model.OrderMessage) error {
			log.Printf("Processing order message: OrderID=%s, Status=%d", order.OrderId, order.Status)

			// 根据订单状态处理
			switch order.Status {
			case model.OrderStatusCreated:
				// 订单创建成功处理
				log.Printf("Order created: %s, triggering follow-up actions", order.OrderId)

			case model.OrderStatusPaid:
				// 支付成功处理
				log.Printf("Order paid: %s, updating order status", order.OrderId)

			case model.OrderStatusPaymentFailed:
				// 支付失败处理
				log.Printf("Order payment failed: %s, need to restore stock", order.OrderId)
			}

			return nil
		})
		if err != nil {
			log.Printf("Order consumer failed: %v", err)
		}
	}()
}

// StartPaymentConsumer 启动支付消息消费者
func (gs *GoodService) StartPaymentConsumer() {
	go func() {
		log.Println("Starting payment message consumer...")
		// 消费支付消息
		err := gs.KafkaRepo.ConsumePaymentMessages(context.Background(), func(orderId string, status int32) error {
			log.Printf("Processing payment message: OrderID=%s, Status=%d", orderId, status)

			// 根据支付状态处理
			switch status {
			case model.OrderStatusPaid:
				// 支付成功处理
				log.Printf("Payment successful for order: %s", orderId)

			case model.OrderStatusPaymentFailed:
				// 支付失败处理
				log.Printf("Payment failed for order: %s, restoring stock...", orderId)
			}

			return nil
		})
		if err != nil {
			log.Printf("Payment consumer failed: %v", err)
		}
	}()
}

// ResetDataBase 重置数据库
func (gs *GoodService) ResetDataBase(goodsId int) error {
	return gs.GoodDB.ResetDataBase(goodsId)
}

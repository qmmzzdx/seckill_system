package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"seckill_system/model"
	"seckill_system/repository"
	"time"

	"gorm.io/gorm"
)

// SeckillHandler 秒杀业务处理器
type SeckillHandler struct {
	redisRepo *repository.RedisRepository // Redis仓库操作
	goodRepo  *repository.GoodRepository  // 商品仓库操作
	kafkaRepo *repository.KafkaRepository // Kafka仓库操作
}

// NewSeckillHandler 创建秒杀处理器实例
func NewSeckillHandler() *SeckillHandler {
	return &SeckillHandler{
		redisRepo: repository.NewRedisRepository(),
		goodRepo:  repository.NewGoodRepository(),
		kafkaRepo: repository.NewKafkaRepository(),
	}
}

// CheckStock 检查商品库存
func (h *SeckillHandler) CheckStock(ctx context.Context, goodsId int64) (int64, error) {
	return h.redisRepo.GetGoodsStock(goodsId)
}

// CreateOrder 创建秒杀订单
func (h *SeckillHandler) CreateOrder(ctx context.Context, userId, goodsId int64) (string, error) {
	// 生成唯一订单ID
	orderId := generateOrderId(userId, goodsId)

	// defer确保Redis库存恢复
	var stockRestored bool
	defer func() {
		if stockRestored {
			h.redisRepo.IncrGoodsStock(goodsId)
		}
	}()

	// 第一步：在Redis中预扣减库存
	remaining, err := h.redisRepo.DecrGoodsStock(goodsId)
	if err != nil {
		return "", fmt.Errorf("decrease goods stock failed: %v", err)
	}

	// 检查库存是否充足
	if remaining < 0 {
		// 库存不足，恢复预扣减的库存
		h.redisRepo.IncrGoodsStock(goodsId)
		return "", errors.New("goods sold out")
	}

	// 在事务中执行数据库操作
	err = h.goodRepo.WithTransaction(func(tx *gorm.DB) error {
		// 获取秒杀活动信息
		promotion, err := h.goodRepo.GetPromotionByGoodsId(goodsId)
		if err != nil {
			return fmt.Errorf("get promotion failed: %v", err)
		}

		// 使用乐观锁扣减数据库库存
		rowsAffected, err := h.goodRepo.OccReduceOnePromotionByGoodsId(goodsId, promotion.Version)
		if err != nil {
			return fmt.Errorf("reduce promotion count failed: %v", err)
		}

		// 检查库存扣减是否成功
		if rowsAffected == 0 {
			// 库存不足，恢复Redis中的预扣减
			h.redisRepo.IncrGoodsStock(goodsId)
			return errors.New("seckill failed, stock not enough")
		}

		// 创建秒杀成功记录
		order := &model.SuccessKilled{
			GoodsId: goodsId,
			UserId:  userId,
			State:   0, // 0-成功未支付
		}
		if err := h.goodRepo.AddSuccessKilled(tx, order); err != nil {
			return fmt.Errorf("create order failed: %v", err)
		}

		// 发送订单创建消息到Kafka
		orderMsg := &model.OrderMessage{
			OrderId:   orderId,
			UserId:    userId,
			GoodsId:   goodsId,
			Price:     promotion.CurrentPrice,
			Status:    model.OrderStatusCreated, // 订单创建成功
			CreatedAt: time.Now(),
		}
		if err := h.kafkaRepo.SendOrderMessage(context.Background(), orderMsg); err != nil {
			log.Printf("Failed to send order message to Kafka: %v", err)
			// 不返回错误，保证订单创建主流程正常
		}

		log.Printf("Order created successfully: OrderID=%s, UserID=%d, GoodsID=%d",
			orderId, userId, goodsId)

		return nil
	})

	if err != nil {
		stockRestored = true
		return "", err
	}
	return orderId, nil
}

// SimulatePayment 模拟支付处理
func (h *SeckillHandler) SimulatePayment(ctx context.Context, orderId string, success bool) error {
	var status int32
	if success {
		status = model.OrderStatusPaid
		log.Printf("Payment successful for order: %s", orderId)
	} else {
		status = model.OrderStatusPaymentFailed
		log.Printf("Payment failed for order: %s", orderId)
		// 支付失败，记录需要恢复库存
		log.Printf("Need to restore stock for failed payment order: %s", orderId)
	}

	// 发送支付结果消息到Kafka
	if err := h.kafkaRepo.SendPaymentMessage(ctx, orderId, status); err != nil {
		log.Printf("Failed to send payment message to Kafka: %v", err)
		return err
	}

	return nil
}

// generateOrderId 生成唯一订单ID
func generateOrderId(userId, goodsId int64) string {
	// 格式: 用户ID-商品ID-时间戳
	return fmt.Sprintf("%d-%d-%d", userId, goodsId, time.Now().UnixNano())
}

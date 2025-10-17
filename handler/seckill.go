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

	// 第一步：在Redis中预扣减库存
	remaining, err := h.redisRepo.DecrGoodsStock(goodsId)
	if err != nil {
		return "", fmt.Errorf("decrease goods stock failed: %v", err)
	}

	// 库存恢复标志，确保异常时库存恢复
	needRestore := false
	defer func() {
		if needRestore {
			log.Printf("Restoring stock for goods %d due to failed order creation", goodsId)
			if _, restoreErr := h.redisRepo.IncrGoodsStock(goodsId); restoreErr != nil {
				log.Printf("Failed to restore stock for goods %d: %v", goodsId, restoreErr)
			} else {
				log.Printf("Successfully restored stock for goods %d", goodsId)
			}
		}
	}()

	// 检查库存是否充足
	if remaining < 0 {
		needRestore = true // 标记需要恢复库存
		return "", errors.New("goods sold out")
	}

	needRestore = true // 标记需要恢复，后续失败时会恢复

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

		// 发送订单创建消息到Kafka（带重试机制）
		orderMsg := &model.OrderMessage{
			OrderId:   orderId,
			UserId:    userId,
			GoodsId:   goodsId,
			Price:     promotion.CurrentPrice,
			Status:    model.OrderStatusCreated, // 订单创建成功
			CreatedAt: time.Now(),
		}

		// 添加Kafka消息发送重试
		if err := h.sendOrderMessageWithRetry(ctx, orderMsg, 3); err != nil {
			log.Printf("Failed to send order message to Kafka after retries: %v", err)
			// 不返回错误，保证订单创建主流程正常，记录日志即可
		}

		log.Printf("Order created successfully: OrderID=%s, UserID=%d, GoodsID=%d",
			orderId, userId, goodsId)

		return nil
	})

	if err != nil {
		// 事务失败，保持needRestore=true，defer会恢复库存
		return "", err
	}

	// 所有操作成功，取消库存恢复
	needRestore = false
	return orderId, nil
}

// sendOrderMessageWithRetry 带重试的Kafka消息发送
func (h *SeckillHandler) sendOrderMessageWithRetry(ctx context.Context, orderMsg *model.OrderMessage, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := h.kafkaRepo.SendOrderMessage(ctx, orderMsg)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Kafka send attempt %d failed: %v", i+1, err)

		// 指数退避
		backoff := time.Duration(i*i) * time.Second
		select {
		case <-time.After(backoff):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed to send message after %d retries: %v", maxRetries, lastErr)
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
	}

	// 发送支付结果消息到Kafka（带重试）
	if err := h.sendPaymentMessageWithRetry(ctx, orderId, status, 3); err != nil {
		log.Printf("Failed to send payment message to Kafka after retries: %v", err)
		return err
	}
	return nil
}

// sendPaymentMessageWithRetry 带重试的支付消息发送
func (h *SeckillHandler) sendPaymentMessageWithRetry(ctx context.Context, orderId string, status int32, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := h.kafkaRepo.SendPaymentMessage(ctx, orderId, status)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Kafka payment message send attempt %d failed: %v", i+1, err)

		// 指数退避
		backoff := time.Duration(i*i) * time.Second
		select {
		case <-time.After(backoff):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed to send payment message after %d retries: %v", maxRetries, lastErr)
}

// generateOrderId 生成唯一订单ID
func generateOrderId(userId, goodsId int64) string {
	// 格式: 用户ID-商品ID-时间戳
	return fmt.Sprintf("%d-%d-%d", userId, goodsId, time.Now().UnixNano())
}

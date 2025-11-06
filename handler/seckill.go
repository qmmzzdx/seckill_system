package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	orderId := generateOrderId(userId, goodsId)

	// 原子性库存预扣减
	canSeckill, err := h.redisRepo.CheckAndDecrStock(goodsId)
	if err != nil || !canSeckill {
		return "", fmt.Errorf("stock check failed: %v", err)
	}

	// 数据库事务（只包含数据库操作）
	var orderSuccess bool
	err = h.goodRepo.WithTransaction(func(tx *gorm.DB) error {
		// 获取秒杀活动信息
		promotion, err := h.goodRepo.GetPromotionByGoodsId(goodsId)
		if err != nil {
			return fmt.Errorf("get promotion failed: %v", err)
		}

		// 乐观锁扣减库存
		rowsAffected, err := h.goodRepo.OccReduceOnePromotionByGoodsId(goodsId, promotion.Version)
		if err != nil {
			return fmt.Errorf("reduce promotion count failed: %v", err)
		}

		if rowsAffected == 0 {
			return errors.New("seckill failed, stock not enough")
		}

		// 创建秒杀成功记录
		order := &model.SuccessKilled{
			GoodsId: goodsId,
			UserId:  userId,
			State:   0,
		}
		if err := h.goodRepo.AddSuccessKilled(tx, order); err != nil {
			return fmt.Errorf("create order failed: %v", err)
		}

		orderSuccess = true
		slog.Info("Order created in database",
			"order_id", orderId,
			"user_id", userId,
			"goods_id", goodsId,
		)
		return nil
	})

	// 如果数据库事务失败，恢复Redis库存
	if err != nil {
		if _, restoreErr := h.redisRepo.IncrGoodsStock(goodsId); restoreErr != nil {
			slog.Error("Failed to restore stock after db failure",
				"goods_id", goodsId,
				"error", restoreErr,
			)
		}
		return "", err
	}

	// 数据库成功后异步发送消息
	if orderSuccess {
		go h.asyncSendOrderMessage(ctx, orderId, userId, goodsId)
	}

	return orderId, nil
}

// asyncSendOrderMessage 异步发送订单消息
func (h *SeckillHandler) asyncSendOrderMessage(ctx context.Context, orderId string, userId, goodsId int64) {
	promotion, err := h.goodRepo.GetPromotionByGoodsId(goodsId)
	if err != nil {
		slog.Error("Failed to get promotion for async message",
			"order_id", orderId,
			"error", err,
		)
		return
	}

	orderMsg := &model.OrderMessage{
		OrderId:   orderId,
		UserId:    userId,
		GoodsId:   goodsId,
		Price:     promotion.CurrentPrice,
		Status:    model.OrderStatusCreated,
		CreatedAt: time.Now(),
	}

	if err := h.sendOrderMessageWithRetry(ctx, orderMsg, 3); err != nil {
		slog.Error("Failed to send async order message",
			"order_id", orderId,
			"error", err,
		)
	}
}

// sendOrderMessageWithRetry 带重试的Kafka消息发送
func (h *SeckillHandler) sendOrderMessageWithRetry(ctx context.Context, orderMsg *model.OrderMessage, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := h.kafkaRepo.SendOrderMessage(ctx, orderMsg)
		if err == nil {
			slog.Info("Order message sent successfully",
				"order_id", orderMsg.OrderId,
				"attempt", i+1,
			)
			return nil
		}
		lastErr = err
		slog.Warn("Kafka send attempt failed",
			"order_id", orderMsg.OrderId,
			"attempt", i+1,
			"error", err,
		)

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
		slog.Info("Payment successful",
			"order_id", orderId,
		)
	} else {
		status = model.OrderStatusPaymentFailed
		slog.Warn("Payment failed",
			"order_id", orderId,
		)
	}

	// 发送支付结果消息到Kafka（带重试）
	if err := h.sendPaymentMessageWithRetry(ctx, orderId, status, 3); err != nil {
		slog.Error("Failed to send payment message to Kafka after retries",
			"order_id", orderId,
			"error", err,
		)
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
			slog.Info("Payment message sent successfully",
				"order_id", orderId,
				"status", status,
				"attempt", i+1,
			)
			return nil
		}
		lastErr = err
		slog.Warn("Kafka payment message send attempt failed",
			"order_id", orderId,
			"attempt", i+1,
			"error", err,
		)

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

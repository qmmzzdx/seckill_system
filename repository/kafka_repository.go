package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"seckill_system/global"
	"seckill_system/model"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaRepository 封装与Kafka交互的仓库操作
type KafkaRepository struct {
	writer *kafka.Writer // Kafka生产者客户端
	reader *kafka.Reader // Kafka消费者客户端
}

// NewKafkaRepository 创建Kafka仓库实例
func NewKafkaRepository() *KafkaRepository {
	return &KafkaRepository{
		writer: global.KafkaWriter, // 使用全局Kafka生产者
		reader: global.KafkaReader, // 使用全局Kafka消费者
	}
}

// SendOrderMessage 发送订单消息到Kafka
func (k *KafkaRepository) SendOrderMessage(ctx context.Context, order *model.OrderMessage) error {
	// 将订单消息序列化为JSON
	jsonData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order message failed: %v", err)
	}

	// 构造Kafka消息
	msg := kafka.Message{
		Key:   []byte(order.OrderId), // 使用订单ID作为key，确保相同订单的消息路由到同一分区
		Value: jsonData,
		Headers: []kafka.Header{
			{
				Key:   "order_id",
				Value: []byte(order.OrderId), // 在消息头中也存储订单ID
			},
			{
				Key:   "message_type",
				Value: []byte("order"), // 标识消息类型为订单
			},
		},
	}

	// 发送消息
	err = k.writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("send order message failed: %v", err)
	}

	slog.Info("Order message sent to Kafka",
		"order_id", order.OrderId,
		"user_id", order.UserId,
		"goods_id", order.GoodsId,
		"status", order.Status,
	)
	return nil
}

// SendPaymentMessage 发送支付消息到Kafka
func (k *KafkaRepository) SendPaymentMessage(ctx context.Context, orderId string, status int32) error {
	// 构造支付消息结构
	paymentMsg := map[string]any{
		"order_id": orderId,
		"status":   status,
		"time":     time.Now(), // 记录支付时间
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(paymentMsg)
	if err != nil {
		return fmt.Errorf("marshal payment message failed: %v", err)
	}

	// 构造Kafka消息
	msg := kafka.Message{
		Key:   []byte(orderId),
		Value: jsonData,
		Headers: []kafka.Header{
			{
				Key:   "order_id",
				Value: []byte(orderId),
			},
			{
				Key:   "message_type",
				Value: []byte("payment"), // 标识消息类型为支付
			},
		},
	}

	// 发送消息
	err = k.writer.WriteMessages(ctx, msg)
	if err != nil {
		return fmt.Errorf("send payment message failed: %v", err)
	}

	slog.Info("Payment message sent to Kafka",
		"order_id", orderId,
		"status", status,
	)
	return nil
}

// ConsumeOrderMessages 消费订单消息
func (k *KafkaRepository) ConsumeOrderMessages(ctx context.Context, handler func(message model.OrderMessage) error) error {
	// 持续消费消息
	for {
		// 读取消息
		msg, err := k.reader.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("read kafka message failed: %v", err)
		}

		// 反序列化订单消息
		var order model.OrderMessage
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			slog.Warn("Failed to unmarshal order message",
				"error", err,
				"message", string(msg.Value),
				"offset", msg.Offset,
				"partition", msg.Partition,
			)
			continue // 跳过无法解析的消息
		}

		// 记录收到的消息
		slog.Info("Received order message from Kafka",
			"order_id", order.OrderId,
			"user_id", order.UserId,
			"status", order.Status,
			"offset", msg.Offset,
			"partition", msg.Partition,
		)

		// 调用处理函数处理消息
		if err := handler(order); err != nil {
			slog.Error("Handle order message failed",
				"order_id", order.OrderId,
				"error", err,
			)
			// 不返回错误，继续处理下一条消息
		}
	}
}

// ConsumePaymentMessages 消费支付消息（使用独立的消费者组）
func (k *KafkaRepository) ConsumePaymentMessages(ctx context.Context, handler func(orderId string, status int32) error) error {
	// 获取全局配置并创建专门的支付消息消费者
	cfg := global.KafkaReader.Config()
	paymentReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID + "_payment", // 使用不同的消费者组
		MinBytes: 10e3,                     // 最小读取字节数
		MaxBytes: 10e6,                     // 最大读取字节数
	})
	defer paymentReader.Close() // 确保关闭消费者

	// 持续消费消息
	for {
		// 读取消息
		msg, err := paymentReader.ReadMessage(ctx)
		if err != nil {
			return fmt.Errorf("read payment message failed: %v", err)
		}

		// 检查消息类型，只处理支付消息
		messageType := getHeaderValue(msg.Headers, "message_type")
		if messageType != "payment" {
			slog.Info("Skipping non-payment message",
				"message_type", messageType,
				"offset", msg.Offset,
			)
			continue // 跳过非支付消息
		}

		// 反序列化支付消息
		var paymentMsg map[string]any
		if err := json.Unmarshal(msg.Value, &paymentMsg); err != nil {
			slog.Warn("Failed to unmarshal payment message",
				"error", err,
				"offset", msg.Offset,
			)
			continue
		}

		// 提取订单ID和状态
		orderId, _ := paymentMsg["order_id"].(string)
		status, _ := paymentMsg["status"].(float64)

		slog.Info("Received payment message from Kafka",
			"order_id", orderId,
			"status", status,
			"offset", msg.Offset,
			"partition", msg.Partition,
		)

		// 调用处理函数处理消息
		if err := handler(orderId, int32(status)); err != nil {
			slog.Error("Handle payment message failed",
				"order_id", orderId,
				"error", err,
			)
		}
	}
}

// getHeaderValue 从消息头中获取指定键的值
func getHeaderValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

// Close 关闭Kafka生产者和消费者连接
func (k *KafkaRepository) Close() error {
	// 关闭生产者
	if err := k.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer failed: %v", err)
	}
	// 关闭消费者
	if err := k.reader.Close(); err != nil {
		return fmt.Errorf("close kafka reader failed: %v", err)
	}
	slog.Info("Kafka repository closed")
	return nil
}

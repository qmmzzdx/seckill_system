package model

import "time"

// Goods 商品信息表
type Goods struct {
	GoodsId        int64     `gorm:"primaryKey;column:goods_id" json:"goods_id"`                     // 商品ID，主键
	Title          string    `gorm:"size:100;column:title" json:"title"`                             // 商品标题，最大长度100
	SubTitle       string    `gorm:"size:200;column:sub_title" json:"sub_title"`                     // 商品副标题，最大长度200
	OriginalCost   float64   `gorm:"column:original_cost" json:"original_cost"`                      // 商品原价
	CurrentPrice   float64   `gorm:"column:current_price" json:"current_price"`                      // 商品当前价格
	Discount       float64   `gorm:"column:discount" json:"discount"`                                // 商品折扣
	IsFreeDelivery int32     `gorm:"column:is_free_delivery" json:"is_free_delivery"`                // 是否包邮：0-不包邮，1-包邮
	CategoryId     int64     `gorm:"index;column:category_id" json:"category_id"`                    // 商品分类ID，有索引
	LastUpdateTime time.Time `gorm:"autoUpdateTime;column:last_update_time" json:"last_update_time"` // 最后更新时间，自动更新
}

// PromotionSecKill 秒杀活动表
type PromotionSecKill struct {
	PsId         int64     `gorm:"primaryKey;column:ps_id" json:"ps_id"`      // 秒杀活动ID，主键
	GoodsId      int64     `gorm:"index;column:goods_id" json:"goods_id"`     // 商品ID，有索引
	PsCount      int64     `gorm:"column:ps_count" json:"ps_count"`           // 秒杀商品数量
	StartTime    time.Time `gorm:"column:start_time" json:"start_time"`       // 秒杀开始时间
	EndTime      time.Time `gorm:"column:end_time" json:"end_time"`           // 秒杀结束时间
	Status       int32     `gorm:"column:status" json:"status"`               // 秒杀状态：0-未开始，1-进行中，2-已结束
	CurrentPrice float64   `gorm:"column:current_price" json:"current_price"` // 秒杀价格
	Version      int64     `gorm:"column:version" json:"version"`             // 版本号，用于乐观锁控制并发
}

// SuccessKilled 秒杀成功记录表
type SuccessKilled struct {
	GoodsId    int64     `gorm:"primaryKey;column:goods_id" json:"goods_id"`           // 商品ID，联合主键
	UserId     int64     `gorm:"primaryKey;column:user_id" json:"user_id"`             // 用户ID，联合主键
	State      int16     `gorm:"column:state" json:"state"`                            // 秒杀状态：0-成功未支付，1-已支付，2-已取消
	CreateTime time.Time `gorm:"autoCreateTime;column:create_time" json:"create_time"` // 创建时间，自动生成
}

// RedisToken 用户令牌信息（Redis存储）
type RedisToken struct {
	Token     string    `json:"token"`      // 用户认证令牌
	UserId    int64     `json:"user_id"`    // 用户ID
	ExpireAt  time.Time `json:"expire_at"`  // 令牌过期时间
	CreatedAt time.Time `json:"created_at"` // 令牌创建时间
}

// RedisSeckillToken 秒杀令牌信息（Redis存储）
type RedisSeckillToken struct {
	TokenId   string    `json:"token_id"`   // 秒杀令牌ID
	UserId    int64     `json:"user_id"`    // 用户ID
	GoodsId   int64     `json:"goods_id"`   // 商品ID
	ExpireAt  time.Time `json:"expire_at"`  // 令牌过期时间
	CreatedAt time.Time `json:"created_at"` // 令牌创建时间
}

// OrderMessage 订单消息（用于消息队列）
type OrderMessage struct {
	OrderId   string    `json:"order_id"`   // 订单ID
	UserId    int64     `json:"user_id"`    // 用户ID
	GoodsId   int64     `json:"goods_id"`   // 商品ID
	Price     float64   `json:"price"`      // 订单价格
	Status    int32     `json:"status"`     // 订单状态：0-创建成功，1-支付成功，2-支付失败，3-订单取消
	CreatedAt time.Time `json:"created_at"` // 订单创建时间
}

// 订单状态常量
const (
	OrderStatusCreated       = iota // 0: 订单创建成功
	OrderStatusPaid                 // 1: 支付成功
	OrderStatusPaymentFailed        // 2: 支付失败
	OrderStatusCancelled            // 3: 订单取消
)

// ETCDConfig ETCD配置信息
type ETCDConfig struct {
	Key     string `json:"key"`     // 配置键
	Value   string `json:"value"`   // 配置值
	Version int64  `json:"version"` // 配置版本号
}

// TableName 指定Goods模型对应的数据库表名
func (Goods) TableName() string {
	return "goods"
}

// TableName 指定PromotionSecKill模型对应的数据库表名
func (PromotionSecKill) TableName() string {
	return "promotion_seckill"
}

// TableName 指定SuccessKilled模型对应的数据库表名
func (SuccessKilled) TableName() string {
	return "success_killed"
}

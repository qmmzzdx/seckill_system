package test

import (
	"seckill_system/model"
	"time"
)

// Test helpers - 测试辅助函数包，提供创建测试数据的工具函数

// CreateTestGoods 创建测试商品数据
// 参数:
//   - goodsId: 商品ID
//
// 返回:
//   - model.Goods: 填充了测试数据的商品对象
func CreateTestGoods(goodsId int64) model.Goods {
	return model.Goods{
		GoodsId:        goodsId,         // 商品ID
		Title:          "Test Book",     // 商品标题
		SubTitle:       "Test Subtitle", // 商品副标题
		OriginalCost:   100.0,           // 原价
		CurrentPrice:   80.0,            // 当前价格
		Discount:       0.8,             // 折扣率
		IsFreeDelivery: 1,               // 是否包邮 (1-是, 0-否)
		CategoryId:     1,               // 分类ID
		LastUpdateTime: time.Now(),      // 最后更新时间
	}
}

// CreateTestPromotion 创建测试秒杀促销数据
// 参数:
//   - goodsId: 商品ID
//   - stock: 促销库存数量
//
// 返回:
//   - model.PromotionSecKill: 填充了测试数据的秒杀促销对象
func CreateTestPromotion(goodsId int64, stock int64) model.PromotionSecKill {
	now := time.Now()
	return model.PromotionSecKill{
		PsId:         1000 + goodsId,          // 促销ID (基于商品ID生成)
		GoodsId:      goodsId,                 // 关联的商品ID
		PsCount:      stock,                   // 促销库存数量
		StartTime:    now.Add(-1 * time.Hour), // 开始时间 (1小时前)
		EndTime:      now.Add(1 * time.Hour),  // 结束时间 (1小时后)
		Status:       1,                       // 状态 (1-启用)
		CurrentPrice: 50.0,                    // 促销价格
		Version:      0,                       // 版本号 (用于乐观锁)
	}
}

// CreateTestOrder 创建测试订单数据
// 参数:
//   - userId: 用户ID
//   - goodsId: 商品ID
//
// 返回:
//   - model.SuccessKilled: 填充了测试数据的秒杀成功订单对象
func CreateTestOrder(userId, goodsId int64) model.SuccessKilled {
	return model.SuccessKilled{
		GoodsId:    goodsId,    // 商品ID
		UserId:     userId,     // 用户ID
		State:      0,          // 订单状态 (0-待支付)
		CreateTime: time.Now(), // 创建时间
	}
}

package repository

import (
	"errors"
	"fmt"
	"seckill_system/global"
	"seckill_system/model"

	"gorm.io/gorm"
)

// GoodRepository 商品数据访问层
// 负责商品相关数据的数据库操作
type GoodRepository struct {
	db *gorm.DB // 数据库连接实例
}

// NewGoodRepository 创建商品仓库实例
func NewGoodRepository() *GoodRepository {
	return &GoodRepository{
		db: global.DBClient, // 使用全局数据库客户端
	}
}

// ResetDataBase 重置数据库数据
// 清除指定商品的订单记录并重置促销库存
func (dao *GoodRepository) ResetDataBase(goodsId int) error {
	return dao.WithTransaction(func(tx *gorm.DB) error {
		// 参数验证
		if goodsId <= 0 {
			return fmt.Errorf("invalid goodsId: %d", goodsId)
		}

		// 清除指定商品的所有订单记录
		if err := dao.ClearOrderByGoodsId(tx, int64(goodsId)); err != nil {
			return fmt.Errorf("failed to clear orders: %w", err)
		}

		// 验证商品是否存在
		if _, err := dao.FindGoodById(int64(goodsId)); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("goods not found: %d", goodsId)
			}
			return fmt.Errorf("failed to find goods: %w", err)
		}

		// 重置指定商品的促销库存数量
		if err := dao.ResetPromotionCountByGoodsId(tx, int64(goodsId), int64(global.BookStockCount)); err != nil {
			return fmt.Errorf("failed to reset promotion count: %w", err)
		}

		return nil
	})
}

// FindGoodById 根据商品ID查询商品信息
func (dao *GoodRepository) FindGoodById(goodsId int64) (model.Goods, error) {
	var good model.Goods
	// 根据goods_id查询商品信息
	err := dao.db.Where("goods_id = ?", goodsId).First(&good).Error
	return good, err
}

// GetPromotionByGoodsId 根据商品ID获取秒杀促销信息
func (dao *GoodRepository) GetPromotionByGoodsId(goodsId int64) (model.PromotionSecKill, error) {
	var promotion model.PromotionSecKill
	// 根据goods_id查询促销信息
	err := dao.db.Where("goods_id = ?", goodsId).First(&promotion).Error
	return promotion, err
}

// OccReduceOnePromotionByGoodsId 使用乐观锁减少促销库存数量
// 通过版本号控制并发安全，防止超卖
func (dao *GoodRepository) OccReduceOnePromotionByGoodsId(goodsId int64, version int64) (int64, error) {
	// 更新促销库存：库存减1，版本号加1
	result := dao.db.Model(&model.PromotionSecKill{}).
		Where("goods_id = ? AND version = ?", goodsId, version). // 版本号匹配条件
		Updates(map[string]any{
			"ps_count": gorm.Expr("ps_count - 1"), // 库存减1
			"version":  gorm.Expr("version + 1"),  // 版本号加1
		})
	// 返回受影响的行数和错误信息
	return result.RowsAffected, result.Error
}

// AddSuccessKilled 添加秒杀成功记录
// 在事务中创建秒杀成功订单
func (dao *GoodRepository) AddSuccessKilled(tx *gorm.DB, order *model.SuccessKilled) error {
	return tx.Create(order).Error
}

// ClearOrderByGoodsId 清除指定商品的所有订单记录
func (dao *GoodRepository) ClearOrderByGoodsId(tx *gorm.DB, goodsId int64) error {
	// 删除指定商品的所有秒杀成功记录
	return tx.Where("goods_id = ?", goodsId).Delete(&model.SuccessKilled{}).Error
}

// ResetPromotionCountByGoodsId 重置指定商品的促销库存数量
func (dao *GoodRepository) ResetPromotionCountByGoodsId(tx *gorm.DB, goodsId int64, count int64) error {
	// 重置库存数量和版本号
	return tx.Model(&model.PromotionSecKill{}).
		Where("goods_id = ?", goodsId).
		Updates(map[string]any{
			"ps_count": count, // 重置库存数量
			"version":  0,     // 重置版本号
		}).Error
}

// WithTransaction 执行数据库事务
// 传入的事务函数会在事务中执行
func (dao *GoodRepository) WithTransaction(fn func(tx *gorm.DB) error) error {
	return dao.db.Transaction(fn)
}

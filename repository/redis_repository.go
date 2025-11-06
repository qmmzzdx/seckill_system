package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"seckill_system/global"
	"seckill_system/model"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisRepository Redis缓存仓库层
// 负责用户令牌、秒杀令牌、库存管理、限流等缓存操作
type RedisRepository struct {
	client *redis.ClusterClient // Redis集群客户端
}

// 包级变量，存储所有Lua脚本
var (
	userRateLimitScript   *redis.Script
	stockOperationsScript *redis.Script
)

// init 函数在包初始化时自动调用，用于加载Lua脚本
func init() {
	// 加载用户限流脚本
	rateLimitScript, err := loadLuaScript("user_rate_limit.lua")
	if err != nil {
		slog.Error("Failed to load user rate limit Lua script", "error", err)
		panic(fmt.Sprintf("Failed to load user rate limit Lua script: %v", err))
	}
	userRateLimitScript = redis.NewScript(rateLimitScript)

	// 加载库存操作脚本
	stockScript, err := loadLuaScript("stock_operations.lua")
	if err != nil {
		slog.Error("Failed to load stock operations Lua script", "error", err)
		panic(fmt.Sprintf("Failed to load stock operations Lua script: %v", err))
	}
	stockOperationsScript = redis.NewScript(stockScript)

	slog.Info("All Lua scripts loaded successfully")
}

// NewRedisRepository 创建Redis仓库实例
func NewRedisRepository() *RedisRepository {
	return &RedisRepository{
		client: global.RedisClusterClient,
	}
}

// loadLuaScript 从文件加载Lua脚本
func loadLuaScript(filename string) (string, error) {
	// 获取当前文件所在目录
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to get current file path")
	}

	// 构建脚本文件路径（脚本文件在项目的scripts目录下）
	scriptPath := filepath.Join(filepath.Dir(currentFile), "..", "scripts", filename)

	// 读取脚本文件内容
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Lua script file %s: %v", scriptPath, err)
	}
	slog.Info("Lua script loaded from file", "path", scriptPath, "filename", filename)
	return string(content), nil
}

// CheckAndDecrStock 原子性地检查并减少库存
func (r *RedisRepository) CheckAndDecrStock(goodsId int64) (bool, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)

	result, err := stockOperationsScript.Run(
		context.Background(),
		r.client,
		[]string{key},
		"check_and_decr", // 命令参数
	).Result()

	if err != nil {
		return false, fmt.Errorf("atomic stock decrease failed: %v", err)
	}

	switch result.(int64) {
	case -1:
		return false, errors.New("goods stock not found")
	case -2:
		return false, errors.New("goods sold out")
	case -99:
		return false, errors.New("unknown stock operation command")
	default:
		slog.Info("Stock decreased atomically",
			"goods_id", goodsId,
			"remaining_stock", result.(int64),
		)
		return true, nil
	}
}

// CheckAndSetStock 原子性地检查并设置库存（如果不存在）
func (r *RedisRepository) CheckAndSetStock(goodsId, stock int64) (bool, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)

	result, err := stockOperationsScript.Run(
		context.Background(),
		r.client,
		[]string{key},
		"check_and_set", // 命令参数
		stock,           // 库存数量
	).Result()

	if err != nil {
		return false, fmt.Errorf("atomic stock set failed: %v", err)
	}

	success := result.(int64) == 1
	if success {
		slog.Info("Stock set atomically",
			"goods_id", goodsId,
			"stock", stock,
		)
	} else {
		slog.Info("Stock already exists, set operation skipped",
			"goods_id", goodsId,
		)
	}
	return success, nil
}

// GetStockAtomic 原子性地获取库存
func (r *RedisRepository) GetStockAtomic(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)

	result, err := stockOperationsScript.Run(
		context.Background(),
		r.client,
		[]string{key},
		"get_stock", // 命令参数
	).Result()

	if err != nil {
		return 0, fmt.Errorf("atomic stock get failed: %v", err)
	}

	stock, err := strconv.ParseInt(result.(string), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse stock result failed: %v", err)
	}

	slog.Info("Stock retrieved atomically",
		"goods_id", goodsId,
		"stock", stock,
	)
	return stock, nil
}

// GenerateUserToken 生成用户认证令牌并存储到Redis
// 令牌有效期为24小时
func (r *RedisRepository) GenerateUserToken(userId int64) (string, error) {
	// 生成随机令牌字符串
	token, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("generate secure token failed: %v", err)
	}
	expireAt := time.Now().Add(24 * time.Hour)

	// 构建令牌数据结构
	tokenData := model.RedisToken{
		Token:     token,
		UserId:    userId,
		ExpireAt:  expireAt,
		CreatedAt: time.Now(),
	}

	// 序列化令牌数据为JSON
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return "", fmt.Errorf("marshal token data failed: %v", err)
	}

	// 存储令牌到Redis，设置过期时间
	key := fmt.Sprintf("user_token:%s", token)
	err = r.client.Set(context.Background(), key, jsonData, time.Until(expireAt)).Err()
	if err != nil {
		return "", fmt.Errorf("store token to redis failed: %v", err)
	}

	slog.Info("User token generated",
		"user_id", userId,
		"token_prefix", token[:8],
		"expire_at", expireAt,
	)
	return token, nil
}

// VerifyUserToken 验证用户令牌有效性并返回用户ID
func (r *RedisRepository) VerifyUserToken(token string) (int64, error) {
	key := fmt.Sprintf("user_token:%s", token)
	data, err := r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Warn("User token not found", "token_prefix", token[:8])
			return 0, errors.New("token not found")
		}
		return 0, fmt.Errorf("get token from redis failed: %v", err)
	}

	// 反序列化令牌数据
	var tokenData model.RedisToken
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return 0, fmt.Errorf("unmarshal token data failed: %v", err)
	}

	// 检查令牌是否过期
	if time.Now().After(tokenData.ExpireAt) {
		r.client.Del(context.Background(), key) // 删除过期令牌
		slog.Warn("User token expired", "token_prefix", token[:8], "user_id", tokenData.UserId)
		return 0, errors.New("token expired")
	}

	slog.Info("User token verified successfully",
		"user_id", tokenData.UserId,
		"token_prefix", token[:8],
	)
	return tokenData.UserId, nil
}

// GenerateSeckillToken 生成秒杀令牌并存储到Redis
// 令牌有效期为30分钟，用于控制秒杀请求
func (r *RedisRepository) GenerateSeckillToken(userId, goodsId int64) (string, error) {
	tokenId, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("generate secure token failed: %v", err)
	}
	expireAt := time.Now().Add(30 * time.Minute)

	// 构建秒杀令牌数据结构
	tokenData := model.RedisSeckillToken{
		TokenId:   tokenId,
		UserId:    userId,
		GoodsId:   goodsId,
		ExpireAt:  expireAt,
		CreatedAt: time.Now(),
	}

	// 序列化秒杀令牌数据
	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return "", fmt.Errorf("marshal seckill token failed: %v", err)
	}

	// 存储秒杀令牌到Redis
	key := fmt.Sprintf("seckill_token:%s", tokenId)
	err = r.client.Set(context.Background(), key, jsonData, time.Until(expireAt)).Err()
	if err != nil {
		return "", fmt.Errorf("store seckill token to redis failed: %v", err)
	}

	slog.Info("Seckill token generated",
		"user_id", userId,
		"goods_id", goodsId,
		"token_id_prefix", tokenId[:8],
		"expire_at", expireAt,
	)
	return tokenId, nil
}

// VerifySeckillToken 验证秒杀令牌有效性
// 验证成功后令牌会被删除（一次性使用）
func (r *RedisRepository) VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error) {
	key := fmt.Sprintf("seckill_token:%s", tokenId)
	data, err := r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
			slog.Warn("Seckill token not found", "token_id_prefix", tokenId[:8])
			return false, nil // 令牌不存在
		}
		return false, fmt.Errorf("get seckill token from redis failed: %v", err)
	}

	// 反序列化秒杀令牌数据
	var tokenData model.RedisSeckillToken
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return false, fmt.Errorf("unmarshal seckill token failed: %v", err)
	}

	// 检查令牌是否过期
	if time.Now().After(tokenData.ExpireAt) {
		r.client.Del(context.Background(), key) // 删除过期令牌
		slog.Warn("Seckill token expired",
			"token_id_prefix", tokenId[:8],
			"user_id", userId,
			"goods_id", goodsId,
		)
		return false, errors.New("token expired")
	}

	// 验证用户ID和商品ID是否匹配
	if tokenData.UserId != userId || tokenData.GoodsId != goodsId {
		slog.Warn("Seckill token mismatch",
			"token_id_prefix", tokenId[:8],
			"expected_user", userId,
			"actual_user", tokenData.UserId,
			"expected_goods", goodsId,
			"actual_goods", tokenData.GoodsId,
		)
		return false, errors.New("token mismatch")
	}

	// 验证成功后删除令牌（防止重复使用）
	r.client.Del(context.Background(), key)

	slog.Info("Seckill token verified and consumed",
		"token_id_prefix", tokenId[:8],
		"user_id", userId,
		"goods_id", goodsId,
	)
	return true, nil
}

// UserRateLimit 用户请求频率限制
// 使用预加载的Lua脚本实现原子性的限流检查
func (r *RedisRepository) UserRateLimit(userId int64, limit int64, duration time.Duration) (bool, error) {
	key := fmt.Sprintf("user_rate_limit:%d", userId)

	// 使用预加载的Lua脚本执行限流逻辑
	result, err := userRateLimitScript.Run(context.Background(), r.client, []string{key}, limit, int(duration.Seconds())).Result()

	if err != nil {
		return false, fmt.Errorf("execute rate limit script failed: %v", err)
	}

	allowed := result.(int64) == 1
	if !allowed {
		slog.Info("User rate limit exceeded",
			"user_id", userId,
			"limit", limit,
			"duration", duration,
		)
	} else {
		slog.Info("User rate limit check passed",
			"user_id", userId,
		)
	}
	return allowed, nil
}

// SetGoodsStock 设置商品库存到Redis
func (r *RedisRepository) SetGoodsStock(goodsId int64, stock int64) error {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	err := r.client.Set(context.Background(), key, stock, 0).Err() // 0表示永不过期
	if err != nil {
		return err
	}

	slog.Info("Goods stock set in Redis",
		"goods_id", goodsId,
		"stock", stock,
	)
	return nil
}

// GetGoodsStock 从Redis获取商品库存
func (r *RedisRepository) GetGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	result, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		if err == redis.Nil {
			slog.Warn("Goods stock not found in Redis", "goods_id", goodsId)
			return 0, nil // key不存在时返回0
		}
		return 0, err
	}

	stock, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return 0, err
	}

	slog.Info("Goods stock retrieved from Redis",
		"goods_id", goodsId,
		"stock", stock,
	)
	return stock, nil
}

// DecrGoodsStock 减少商品库存（原子操作）
// 返回减少后的库存值
func (r *RedisRepository) DecrGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	result, err := r.client.Decr(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}

	slog.Info("Goods stock decreased",
		"goods_id", goodsId,
		"remaining_stock", result,
	)
	return result, nil
}

// IncrGoodsStock 增加商品库存（原子操作）
// 返回增加后的库存值
func (r *RedisRepository) IncrGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	result, err := r.client.Incr(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}

	slog.Info("Goods stock increased",
		"goods_id", goodsId,
		"current_stock", result,
	)
	return result, nil
}

// generateRandomString 生成指定长度的随机字符串
// 用于生成令牌ID等随机标识
func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %v", err)
	}
	for i := range bytes {
		bytes[i] = charset[bytes[i]%byte(len(charset))]
	}
	return string(bytes), nil
}

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
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

// 包级变量，存储用户限流Lua脚本
var userRateLimitScript *redis.Script

// init 函数在包初始化时自动调用，用于加载Lua脚本
func init() {
	script, err := loadLuaScript("user_rate_limit.lua")
	if err != nil {
		panic(fmt.Sprintf("Failed to load Lua script: %v", err))
	}
	userRateLimitScript = redis.NewScript(script)
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

	// 构建脚本文件路径（假设脚本文件在项目的scripts目录下）
	scriptPath := filepath.Join(filepath.Dir(currentFile), "..", "scripts", filename)

	// 读取脚本文件内容
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Lua script file %s: %v", scriptPath, err)
	}

	return string(content), nil
}

// GenerateUserToken 生成用户认证令牌并存储到Redis
// 令牌有效期为24小时
func (r *RedisRepository) GenerateUserToken(userId int64) (string, error) {
	// 生成随机令牌字符串
	token := generateRandomString(32)
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

	return token, nil
}

// VerifyUserToken 验证用户令牌有效性并返回用户ID
func (r *RedisRepository) VerifyUserToken(token string) (int64, error) {
	key := fmt.Sprintf("user_token:%s", token)
	data, err := r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
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
		return 0, errors.New("token expired")
	}

	return tokenData.UserId, nil
}

// GenerateSeckillToken 生成秒杀令牌并存储到Redis
// 令牌有效期为30分钟，用于控制秒杀请求
func (r *RedisRepository) GenerateSeckillToken(userId, goodsId int64) (string, error) {
	tokenId := generateRandomString(32)
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

	return tokenId, nil
}

// VerifySeckillToken 验证秒杀令牌有效性
// 验证成功后令牌会被删除（一次性使用）
func (r *RedisRepository) VerifySeckillToken(tokenId string, userId, goodsId int64) (bool, error) {
	key := fmt.Sprintf("seckill_token:%s", tokenId)
	data, err := r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if err == redis.Nil {
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
		return false, errors.New("token expired")
	}

	// 验证用户ID和商品ID是否匹配
	if tokenData.UserId != userId || tokenData.GoodsId != goodsId {
		return false, errors.New("token mismatch")
	}

	// 验证成功后删除令牌（防止重复使用）
	r.client.Del(context.Background(), key)
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

	return result.(int64) == 1, nil
}

// SetGoodsStock 设置商品库存到Redis
func (r *RedisRepository) SetGoodsStock(goodsId int64, stock int64) error {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	return r.client.Set(context.Background(), key, stock, 0).Err() // 0表示永不过期
}

// GetGoodsStock 从Redis获取商品库存
func (r *RedisRepository) GetGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	result, err := r.client.Get(context.Background(), key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil // key不存在时返回0
		}
		return 0, err
	}
	return strconv.ParseInt(result, 10, 64)
}

// DecrGoodsStock 减少商品库存（原子操作）
// 返回减少后的库存值
func (r *RedisRepository) DecrGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	return r.client.Decr(context.Background(), key).Result()
}

// IncrGoodsStock 增加商品库存（原子操作）
// 返回增加后的库存值
func (r *RedisRepository) IncrGoodsStock(goodsId int64) (int64, error) {
	key := fmt.Sprintf("goods_stock:%d", goodsId)
	return r.client.Incr(context.Background(), key).Result()
}

// generateRandomString 生成指定长度的随机字符串
// 用于生成令牌ID等随机标识
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

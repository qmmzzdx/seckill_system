package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"seckill_system/global"
	"seckill_system/model"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ETCDRepository 封装与ETCD交互的仓库操作
type ETCDRepository struct {
	client *clientv3.Client // ETCD客户端实例
}

// NewETCDRepository 创建ETCD仓库实例
func NewETCDRepository() *ETCDRepository {
	return &ETCDRepository{
		client: global.EtcdClient, // 使用全局ETCD客户端
	}
}

// GetSeckillEnabled 获取秒杀开关状态
func (e *ETCDRepository) GetSeckillEnabled(ctx context.Context) (bool, error) {
	// 从ETCD获取秒杀开关配置
	resp, err := e.client.Get(ctx, global.EtcdKeySeckillEnabled)
	if err != nil {
		return false, fmt.Errorf("get seckill enabled failed: %v", err)
	}

	// 如果不存在配置项，默认返回true(开启状态)
	if len(resp.Kvs) == 0 {
		return true, nil
	}

	// 解析配置值
	enabled := string(resp.Kvs[0].Value)
	return enabled == "true", nil
}

// SetSeckillEnabled 设置秒杀开关状态
func (e *ETCDRepository) SetSeckillEnabled(ctx context.Context, enabled bool) error {
	// 根据输入参数设置对应的字符串值
	value := "false"
	if enabled {
		value = "true"
	}

	// 写入ETCD
	_, err := e.client.Put(ctx, global.EtcdKeySeckillEnabled, value)
	if err != nil {
		return fmt.Errorf("set seckill enabled failed: %v", err)
	}
	return nil
}

// GetRateLimitConfig 获取限流配置
func (e *ETCDRepository) GetRateLimitConfig(ctx context.Context) (int64, error) {
	// 从ETCD获取限流配置
	resp, err := e.client.Get(ctx, global.EtcdKeyRateLimit)
	if err != nil {
		return 10, fmt.Errorf("get rate limit config failed: %v", err) // 默认返回5次/分钟
	}

	// 如果不存在配置项，返回默认值
	if len(resp.Kvs) == 0 {
		return 10, nil
	}

	// 解析配置值
	limit, err := strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64)
	if err != nil {
		return 10, nil // 解析失败返回默认值
	}

	return limit, nil
}

// SetRateLimitConfig 设置限流配置
func (e *ETCDRepository) SetRateLimitConfig(ctx context.Context, limit int64) error {
	// 将限流值转换为字符串并写入ETCD
	_, err := e.client.Put(ctx, global.EtcdKeyRateLimit, strconv.FormatInt(limit, 10))
	if err != nil {
		return fmt.Errorf("set rate limit config failed: %v", err)
	}
	return nil
}

// AddToBlacklist 添加用户到黑名单
func (e *ETCDRepository) AddToBlacklist(ctx context.Context, userId int64, reason string, duration time.Duration) error {
	// 构造黑名单键名
	key := fmt.Sprintf("%s%d", global.EtcdKeyBlacklist, userId)

	// 构造黑名单信息结构
	blacklistInfo := map[string]any{
		"user_id":  userId,
		"reason":   reason,
		"add_time": time.Now().Format(time.RFC3339),
		"expire":   time.Now().Add(duration).Format(time.RFC3339),
	}

	// 序列化为JSON
	data, err := json.Marshal(blacklistInfo)
	if err != nil {
		return fmt.Errorf("marshal blacklist info failed: %v", err)
	}

	// 创建租约实现自动过期
	leaseResp, err := e.client.Grant(ctx, int64(duration.Seconds()))
	if err != nil {
		return fmt.Errorf("grant lease failed: %v", err)
	}

	// 写入ETCD并关联租约
	_, err = e.client.Put(ctx, key, string(data), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return fmt.Errorf("add to blacklist failed: %v", err)
	}

	log.Printf("User %d added to blacklist, reason: %s, duration: %v", userId, reason, duration)
	return nil
}

// RemoveFromBlacklist 从黑名单移除用户
func (e *ETCDRepository) RemoveFromBlacklist(ctx context.Context, userId int64) error {
	// 构造键名并删除
	key := fmt.Sprintf("%s%d", global.EtcdKeyBlacklist, userId)
	_, err := e.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("remove from blacklist failed: %v", err)
	}
	return nil
}

// IsInBlacklist 检查用户是否在黑名单中
func (e *ETCDRepository) IsInBlacklist(ctx context.Context, userId int64) (bool, error) {
	// 构造键名并查询
	key := fmt.Sprintf("%s%d", global.EtcdKeyBlacklist, userId)
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check blacklist failed: %v", err)
	}

	// 根据是否存在键值判断是否在黑名单中
	return len(resp.Kvs) > 0, nil
}

// GetBlacklist 获取黑名单列表
func (e *ETCDRepository) GetBlacklist(ctx context.Context) ([]map[string]any, error) {
	// 使用前缀查询获取所有黑名单条目
	resp, err := e.client.Get(ctx, global.EtcdKeyBlacklist, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get blacklist failed: %v", err)
	}

	var blacklist []map[string]any
	for _, kv := range resp.Kvs {
		var info map[string]any
		// 反序列化JSON数据
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			log.Printf("Failed to unmarshal blacklist info: %v", err)
			continue
		}
		blacklist = append(blacklist, info)
	}

	return blacklist, nil
}

// WatchSeckillConfig 监听秒杀配置变化
func (e *ETCDRepository) WatchSeckillConfig(ctx context.Context, callback func(key, value string)) {
	// 创建监听通道
	rch := e.client.Watch(ctx, global.EtcdKeySeckillEnabled, clientv3.WithPrefix())

	// 启动goroutine处理监听事件
	go func() {
		for wresp := range rch {
			for _, ev := range wresp.Events {
				log.Printf("Etcd config changed: %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				if callback != nil {
					callback(string(ev.Kv.Key), string(ev.Kv.Value))
				}
			}
		}
	}()
}

// GetDistributedLock 获取分布式锁
func (e *ETCDRepository) GetDistributedLock(ctx context.Context, key string, ttl int) (bool, error) {
	// 创建租约
	lease, err := e.client.Grant(ctx, int64(ttl))
	if err != nil {
		return false, fmt.Errorf("grant lease failed: %v", err)
	}

	// 使用事务实现原子操作
	resp, err := e.client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).        // 检查key不存在
		Then(clientv3.OpPut(key, "locked", clientv3.WithLease(lease.ID))). // 写入锁
		Commit()
	if err != nil {
		return false, fmt.Errorf("etcd transaction failed: %v", err)
	}

	return resp.Succeeded, nil
}

// ReleaseDistributedLock 释放分布式锁
func (e *ETCDRepository) ReleaseDistributedLock(ctx context.Context, key string) error {
	// 删除锁键
	_, err := e.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("delete etcd key failed: %v", err)
	}
	return nil
}

// PutConfig 存储配置
func (e *ETCDRepository) PutConfig(ctx context.Context, key, value string) error {
	// 简单写入配置
	_, err := e.client.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("put etcd config failed: %v", err)
	}
	return nil
}

// GetConfig 获取配置
func (e *ETCDRepository) GetConfig(ctx context.Context, key string) (model.ETCDConfig, error) {
	// 获取配置值
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return model.ETCDConfig{}, fmt.Errorf("get etcd config failed: %v", err)
	}

	// 处理空结果
	if len(resp.Kvs) == 0 {
		return model.ETCDConfig{}, nil
	}

	// 返回配置结构
	return model.ETCDConfig{
		Key:     key,
		Value:   string(resp.Kvs[0].Value),
		Version: resp.Kvs[0].Version,
	}, nil
}

// Close 关闭ETCD客户端连接
func (e *ETCDRepository) Close() error {
	if err := e.client.Close(); err != nil {
		return fmt.Errorf("close etcd client failed: %v", err)
	}
	return nil
}

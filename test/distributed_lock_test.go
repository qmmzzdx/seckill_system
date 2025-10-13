package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestETCDRepository_DistributedLock 测试ETCD分布式锁功能
func TestETCDRepository_DistributedLock(t *testing.T) {
	// 创建模拟ETCD仓库实例
	mockETCD := NewMockETCDRepository()

	// 使用反射或其他方式替换client
	// 实际可以使用接口和依赖注入

	// 创建上下文
	ctx := context.Background()

	// 第一次获取锁应该成功
	locked, err := mockETCD.GetDistributedLock(ctx, "test-key", 10)
	assert.NoError(t, err) // 验证没有错误发生
	assert.True(t, locked) // 验证成功获取到锁

	// 第二次获取相同锁应该失败（锁已被占用）
	locked, err = mockETCD.GetDistributedLock(ctx, "test-key", 10)
	assert.NoError(t, err)  // 验证没有错误发生
	assert.False(t, locked) // 验证获取锁失败（因为锁已被占用）

	// 释放锁
	err = mockETCD.ReleaseDistributedLock(ctx, "test-key")
	assert.NoError(t, err) // 验证释放锁操作没有错误

	// 释放后可以重新获取锁
	locked, err = mockETCD.GetDistributedLock(ctx, "test-key", 10)
	assert.NoError(t, err) // 验证没有错误发生
	assert.True(t, locked) // 验证成功重新获取到锁
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"seckill_system/config"
	"seckill_system/global"
	"seckill_system/web/router"
)

// 初始化全局变量
func init() {
	global.DBClient = nil
	global.RedisClusterClient = nil
	global.KafkaWriter = nil
	global.KafkaReader = nil
	global.EtcdClient = nil
}

// 程序主入口
func main() {
	// 加载配置文件
	config.InitConfig("conf/conf.yaml")
	cfg := config.AppConfig

	// 初始化数据库和中间件连接
	global.InitMySQL()
	global.InitRedis()
	global.InitKafka()
	global.InitEtcd()

	// 设置路由
	gateway := router.InitRouter()

	// 配置HTTP服务器
	gatewayServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: gateway,
	}

	// 启动HTTP服务
	go func() {
		slog.Info("🚀 Seckill system gateway service started",
			"port", cfg.Server.Port,
		)
		if err := gatewayServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Seckill system gateway service failed", "error", err)
			os.Exit(1)
		}
	}()

	// 监听终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// 设置优雅关闭超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	if err := gatewayServer.Shutdown(ctx); err != nil {
		slog.Error("Gateway forced to shutdown", "error", err)
	} else {
		slog.Info("Gateway gracefully stopped")
	}

	// 释放所有资源
	cleanupResources()
	slog.Info("Server exited")
}

// 关闭所有服务连接
func cleanupResources() {
	global.CloseMysql()
	global.CloseRedis()
	global.CloseKafka()
	global.CloseEtcd()
}

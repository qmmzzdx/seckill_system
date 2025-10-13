package router

import (
	"seckill_system/web/controller"
	"seckill_system/web/middleware"

	"github.com/gin-gonic/gin"
)

// InitRouter 初始化并返回Gin路由引擎
func InitRouter() *gin.Engine {
	// 创建默认Gin引擎实例
	r := gin.Default()

	// 初始化控制器实例
	goodController := controller.NewGoodController()

	// 创建API路由组，所有接口前缀为/api
	api := r.Group("/api")
	{
		// 认证相关接口
		auth := api.Group("/auth")
		{
			auth.GET("/create_user_token", goodController.GenerateUserToken) // 生成用户令牌接口
			auth.GET("/verify_user_token", goodController.VerifyToken)       // 验证用户令牌接口
		}

		// 商品信息接口 - 获取商品详情
		api.GET("/goods/:id", goodController.GetGoodInfo)

		// 秒杀相关接口
		api.POST("/seckill/token", middleware.AuthMiddleware(), goodController.GetSeckillToken) // 获取秒杀令牌接口
		api.POST("/seckill", middleware.AuthMiddleware(), goodController.SeckillWithToken)      // 使用令牌进行秒杀接口

		// 支付相关接口
		api.POST("/payment/simulate", middleware.AuthMiddleware(), goodController.SimulatePayment) // 模拟支付接口

		// 管理接口组，需要管理员权限
		admin := api.Group("/admin", middleware.AdminMiddleware())
		{
			// 商品库存预加载接口 - 修复：使用路径参数
			admin.POST("/preload/:id", goodController.PreloadGoodsStock)
			// 数据库重置接口
			admin.POST("/reset_db", goodController.ResetDatabase)

			// Etcd配置管理接口
			admin.POST("/config/seckill/enable", goodController.SetSeckillEnabled) // 设置秒杀开关状态
			admin.POST("/config/rate_limit", goodController.SetRateLimit)          // 设置限流配置

			// 黑名单管理接口
			admin.POST("/blacklist/add", goodController.AddToBlacklist) // 添加用户到黑名单
			admin.GET("/blacklist", goodController.GetBlacklist)        // 获取黑名单列表
		}
	}
	return r
}

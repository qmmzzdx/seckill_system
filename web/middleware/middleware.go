package middleware

import (
	"log/slog"
	"net/http"
	"seckill_system/service"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 用户认证中间件
// 验证请求头中的Authorization令牌，解析用户ID并存入上下文
func AuthMiddleware() gin.HandlerFunc {
	// 获取商品服务对象，用于令牌验证
	goodService := service.GetGoodService()

	return func(c *gin.Context) {
		// 从请求头获取Authorization令牌
		token := c.GetHeader("Authorization")
		if token == "" {
			slog.Warn("Missing authorization token in middleware",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)
			// 令牌为空，返回401未授权错误
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    -1,                            // 错误代码
				"error":   "missing authorization token", // 错误详情
				"message": "Authentication required",     // 用户提示信息
			})
			return
		}

		// 验证令牌有效性，获取用户ID
		userId, err := goodService.VerifyUserToken(token)
		if err != nil {
			slog.Warn("Invalid authorization token in middleware",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"token_prefix", token[:8],
				"error", err,
			)
			// 令牌验证失败，返回401未授权错误
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    -1,
				"error":   err.Error(),     // 具体的验证错误信息
				"message": "Invalid token", // 用户提示信息
			})
			return
		}

		// 令牌验证成功，将用户ID存入上下文供后续处理使用
		c.Set("userId", userId)

		slog.Info("User authenticated successfully",
			"user_id", userId,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"token_prefix", token[:8],
		)
		// 继续执行后续的中间件或处理函数
		c.Next()
	}
}

// AdminMiddleware 管理员权限验证中间件
// 简易版管理员验证，通过查询参数检查是否为管理员操作
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查请求参数中是否包含admin=1（当前为简易实现，未做数据库校验）
		if c.Query("admin") != "1" {
			slog.Warn("Admin permission required but not provided",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"client_ip", c.ClientIP(),
			)
			// 非管理员请求，禁止访问
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    -1,                                                  // 错误代码
				"error":   "admin permission required",                         // 错误详情
				"message": "Please add admin=1 parameter for admin operations", // 操作提示
			})
			return
		}

		slog.Info("Admin access granted",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"client_ip", c.ClientIP(),
		)
		// 管理员验证通过，继续执行后续处理
		c.Next()
	}
}

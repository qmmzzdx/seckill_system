package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"seckill_system/service"

	"github.com/gin-gonic/gin"
)

// GoodController 处理商品相关请求的控制器
type GoodController struct {
	GoodService *service.GoodService // 商品服务实例
}

// NewGoodController 创建GoodController实例
func NewGoodController() *GoodController {
	return &GoodController{
		GoodService: service.GetGoodService(),
	}
}

// GetGoodInfo 获取商品信息接口
func (g *GoodController) GetGoodInfo(c *gin.Context) {
	// 从路径参数中获取商品ID
	id := c.Param("id")
	gid, err := strconv.Atoi(id)
	if err != nil {
		slog.Warn("Invalid good ID in request",
			"id", id,
			"error", err,
		)
		// 返回参数错误响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid good ID",
		})
		return
	}

	// 调用服务层获取商品信息
	good, err := g.GoodService.FindGoodById(int64(gid))
	if err != nil {
		slog.Error("Failed to query product data",
			"goods_id", gid,
			"error", err,
		)
		// 返回查询失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to query product data",
		})
		return
	}

	slog.Info("Product data queried successfully",
		"goods_id", gid,
		"title", good.Title,
	)
	// 返回商品信息
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"good_info": good,
		},
		"message": "Product data queried successfully",
	})
}

// GetSeckillToken 获取秒杀令牌接口
func (g *GoodController) GetSeckillToken(c *gin.Context) {
	// 从请求头获取授权令牌
	token := c.GetHeader("Authorization")
	if token == "" {
		slog.Warn("Missing authorization token in request")
		// 返回未授权响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   "missing authorization token",
			"message": "Authentication required",
		})
		return
	}

	// 验证用户令牌
	userId, err := g.GoodService.VerifyUserToken(token)
	if err != nil {
		slog.Warn("Invalid user token",
			"token", token,
			"error", err,
		)
		// 返回令牌无效响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid token",
		})
		return
	}

	// 获取商品ID
	goodsIdStr := c.Query("gid")
	goodsId, err := strconv.ParseInt(goodsIdStr, 10, 64)
	if err != nil {
		slog.Warn("Invalid goods ID in request",
			"user_id", userId,
			"goods_id_str", goodsIdStr,
			"error", err,
		)
		// 返回商品ID无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid good ID",
		})
		return
	}

	// 生成秒杀令牌
	tokenId, err := g.GoodService.GenerateSeckillToken(userId, goodsId)
	if err != nil {
		slog.Error("Failed to generate seckill token",
			"user_id", userId,
			"goods_id", goodsId,
			"error", err,
		)
		// 返回生成令牌失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to generate seckill token",
		})
		return
	}

	slog.Info("Seckill token generated successfully",
		"user_id", userId,
		"goods_id", goodsId,
		"token_id_prefix", tokenId[:8],
	)
	// 返回秒杀令牌
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    gin.H{"token": tokenId},
		"message": "Seckill token generated successfully",
	})
}

// SeckillWithToken 使用令牌进行秒杀接口
func (g *GoodController) SeckillWithToken(c *gin.Context) {
	// 验证用户令牌
	token := c.GetHeader("Authorization")
	if token == "" {
		slog.Warn("Missing authorization token in seckill request")
		// 返回未授权响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   "missing authorization token",
			"message": "Authentication required",
		})
		return
	}

	// 验证用户令牌并获取用户ID
	userId, err := g.GoodService.VerifyUserToken(token)
	if err != nil {
		slog.Warn("Invalid user token in seckill request",
			"token", token,
			"error", err,
		)
		// 返回令牌无效响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid token",
		})
		return
	}

	// 获取商品ID
	goodsIdStr := c.Query("gid")
	goodsId, err := strconv.ParseInt(goodsIdStr, 10, 64)
	if err != nil {
		slog.Warn("Invalid goods ID in seckill request",
			"user_id", userId,
			"goods_id_str", goodsIdStr,
			"error", err,
		)
		// 返回商品ID无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid good ID",
		})
		return
	}

	// 获取秒杀令牌
	tokenId := c.Query("token")
	if tokenId == "" {
		slog.Warn("Missing seckill token in request",
			"user_id", userId,
			"goods_id", goodsId,
		)
		// 返回缺少秒杀令牌响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "missing seckill token",
			"message": "Seckill token required",
		})
		return
	}

	// 执行秒杀操作
	orderId, err := g.GoodService.SeckillWithToken(userId, goodsId, tokenId)
	if err != nil {
		slog.Error("Seckill failed",
			"user_id", userId,
			"goods_id", goodsId,
			"token_id_prefix", tokenId[:8],
			"error", err,
		)
		// 返回秒杀失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Seckill failed",
		})
		return
	}

	slog.Info("Seckill successful via API",
		"user_id", userId,
		"goods_id", goodsId,
		"order_id", orderId,
		"token_id_prefix", tokenId[:8],
	)
	// 返回订单ID
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"data":    gin.H{"order_id": orderId},
		"message": "Seckill success",
	})
}

// SimulatePayment 模拟支付接口
func (g *GoodController) SimulatePayment(c *gin.Context) {
	// 获取订单ID
	orderId := c.Query("order_id")
	if orderId == "" {
		slog.Warn("Missing order_id in payment simulation request")
		// 返回缺少订单ID响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "missing order_id",
			"message": "Order ID required",
		})
		return
	}

	// 获取支付状态参数
	successStr := c.Query("success")
	success, err := strconv.ParseBool(successStr)
	if err != nil {
		success = true // 默认支付成功
		slog.Info("Using default success value for payment simulation",
			"order_id", orderId,
			"success_str", successStr,
		)
	}

	// 执行模拟支付
	err = g.GoodService.SimulatePayment(orderId, success)
	if err != nil {
		slog.Error("Payment simulation failed",
			"order_id", orderId,
			"success", success,
			"error", err,
		)
		// 返回支付模拟失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Payment simulation failed",
		})
		return
	}

	// 返回支付结果
	status := "success"
	if !success {
		status = "failed"
	}

	slog.Info("Payment simulation completed via API",
		"order_id", orderId,
		"status", status,
	)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Payment simulation " + status,
	})
}

// PreloadGoodsStock 预加载商品库存接口
func (g *GoodController) PreloadGoodsStock(c *gin.Context) {
	// 从路径参数中获取商品ID
	id := c.Param("id")
	goodsId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Warn("Invalid goods ID in preload request",
			"id", id,
			"error", err,
		)
		// 返回商品ID无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid good ID",
		})
		return
	}

	// 执行预加载
	err = g.GoodService.PreloadGoodsStock(goodsId)
	if err != nil {
		slog.Error("Failed to preload goods stock",
			"goods_id", goodsId,
			"error", err,
		)
		// 返回预加载失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to preload goods stock",
		})
		return
	}

	slog.Info("Goods stock preloaded successfully via API",
		"goods_id", goodsId,
	)
	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Goods stock preloaded successfully",
	})
}

// SetSeckillEnabled 设置秒杀开关状态接口
func (g *GoodController) SetSeckillEnabled(c *gin.Context) {
	// 获取启用状态参数
	enabledStr := c.Query("enabled")
	enabled, err := strconv.ParseBool(enabledStr)
	if err != nil {
		slog.Warn("Invalid enabled parameter in request",
			"enabled_str", enabledStr,
			"error", err,
		)
		// 返回参数无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "invalid enabled parameter",
			"message": "Enabled parameter must be true or false",
		})
		return
	}

	// 设置秒杀开关状态
	err = g.GoodService.SetSeckillEnabled(enabled)
	if err != nil {
		slog.Error("Failed to set seckill enabled",
			"enabled", enabled,
			"error", err,
		)
		// 返回设置失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to set seckill enabled",
		})
		return
	}

	// 返回设置结果
	status := "enabled"
	if !enabled {
		status = "disabled"
	}

	slog.Info("Seckill system status updated via API",
		"status", status,
	)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Seckill system " + status,
	})
}

// SetRateLimit 设置限流配置接口
func (g *GoodController) SetRateLimit(c *gin.Context) {
	// 获取限流值参数
	limitStr := c.Query("limit")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit <= 0 {
		slog.Warn("Invalid limit parameter in request",
			"limit_str", limitStr,
			"error", err,
		)
		// 返回参数无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "invalid limit parameter",
			"message": "Limit must be a positive integer",
		})
		return
	}

	// 设置限流值
	err = g.GoodService.SetRateLimit(limit)
	if err != nil {
		slog.Error("Failed to set rate limit",
			"limit", limit,
			"error", err,
		)
		// 返回设置失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to set rate limit",
		})
		return
	}

	slog.Info("Rate limit updated via API",
		"limit", limit,
	)
	// 返回设置结果
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Rate limit set to " + limitStr + " requests per minute",
	})
}

// AddToBlacklist 添加用户到黑名单接口
func (g *GoodController) AddToBlacklist(c *gin.Context) {
	// 获取用户ID参数
	userIdStr := c.Query("user_id")
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil || userId <= 0 {
		slog.Warn("Invalid user_id parameter in blacklist request",
			"user_id_str", userIdStr,
			"error", err,
		)
		// 返回参数无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "invalid user_id parameter",
			"message": "User ID must be a positive integer",
		})
		return
	}

	// 获取原因参数
	reason := c.Query("reason")
	if reason == "" {
		reason = "Manual addition" // 默认原因
		slog.Info("Using default reason for blacklist addition",
			"user_id", userId,
		)
	}

	// 获取持续时间参数
	durationStr := c.Query("duration")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 24 * time.Hour // 默认24小时
		slog.Info("Using default duration for blacklist addition",
			"user_id", userId,
			"duration_str", durationStr,
		)
	}

	// 添加用户到黑名单
	err = g.GoodService.AddToBlacklist(userId, reason, duration)
	if err != nil {
		slog.Error("Failed to add user to blacklist",
			"user_id", userId,
			"reason", reason,
			"duration", duration,
			"error", err,
		)
		// 返回添加失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to add user to blacklist",
		})
		return
	}

	slog.Info("User added to blacklist via API",
		"user_id", userId,
		"reason", reason,
		"duration", duration,
	)
	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "User added to blacklist successfully",
	})
}

// GetBlacklist 获取黑名单列表接口
func (g *GoodController) GetBlacklist(c *gin.Context) {
	// 获取黑名单列表
	blacklist, err := g.GoodService.GetBlacklist()
	if err != nil {
		slog.Error("Failed to get blacklist",
			"error", err,
		)
		// 返回获取失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to get blacklist",
		})
		return
	}

	slog.Info("Blacklist retrieved via API",
		"count", len(blacklist),
	)
	// 返回黑名单数据
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"blacklist": blacklist,
		},
		"message": "Blacklist retrieved successfully",
	})
}

// GenerateUserToken 生成用户令牌接口
func (g *GoodController) GenerateUserToken(c *gin.Context) {
	// 从查询参数获取用户ID
	userIdStr := c.Query("user_id")
	if userIdStr == "" {
		slog.Warn("Missing user_id parameter in token generation request")
		// 返回缺少用户ID响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "missing user_id parameter",
			"message": "User ID is required",
		})
		return
	}

	// 解析用户ID
	var userId int64
	_, err := fmt.Sscanf(userIdStr, "%d", &userId)
	if err != nil || userId <= 0 {
		slog.Warn("Invalid user_id parameter in token generation request",
			"user_id_str", userIdStr,
			"error", err,
		)
		// 返回用户ID无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "invalid user_id parameter",
			"message": "User ID must be a positive integer",
		})
		return
	}

	// 生成用户token
	token, err := g.GoodService.GenerateUserToken(userId)
	if err != nil {
		slog.Error("Failed to generate user token",
			"user_id", userId,
			"error", err,
		)
		// 返回生成令牌失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to generate token",
		})
		return
	}

	slog.Info("User token generated successfully via API",
		"user_id", userId,
		"token", token,
	)
	// 返回token
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"user_id": userId,
			"token":   token,
		},
		"message": "Token generated successfully",
	})
}

// VerifyToken 验证令牌接口
func (g *GoodController) VerifyToken(c *gin.Context) {
	// 获取令牌参数
	token := c.Query("token")
	if token == "" {
		slog.Warn("Missing token parameter in verification request")
		// 返回缺少令牌响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "missing token parameter",
			"message": "Token is required",
		})
		return
	}

	// 验证token
	userId, err := g.GoodService.VerifyUserToken(token)
	if err != nil {
		slog.Warn("Token verification failed",
			"token", token,
			"error", err,
		)
		// 返回令牌无效响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid token",
		})
		return
	}

	slog.Info("Token verified successfully via API",
		"user_id", userId,
		"token", token,
	)
	// 返回验证成功响应
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"user_id": userId,
			"valid":   true,
		},
		"message": "Token is valid",
	})
}

// ResetDatabase 重置数据库接口
func (g *GoodController) ResetDatabase(c *gin.Context) {
	// 获取商品ID参数
	goodsIdStr := c.Query("goods_id")
	if goodsIdStr == "" {
		slog.Warn("Missing goods_id parameter in reset request")
		// 返回缺少商品ID响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "missing goods_id parameter",
			"message": "Goods ID is required",
		})
		return
	}

	// 解析商品ID
	goodsId, err := strconv.Atoi(goodsIdStr)
	if err != nil || goodsId <= 0 {
		slog.Warn("Invalid goods_id parameter in reset request",
			"goods_id_str", goodsIdStr,
			"error", err,
		)
		// 返回商品ID无效响应
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    -1,
			"error":   "invalid goods_id parameter",
			"message": "Goods ID must be a positive integer",
		})
		return
	}

	// 执行重置数据库操作
	err = g.GoodService.ResetDataBase(goodsId)
	if err != nil {
		slog.Error("Failed to reset database",
			"goods_id", goodsId,
			"error", err,
		)
		// 返回重置失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to reset database",
		})
		return
	}

	slog.Info("Database reset successfully via API",
		"goods_id", goodsId,
	)
	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Database reset successfully for goods ID: " + goodsIdStr,
	})
}

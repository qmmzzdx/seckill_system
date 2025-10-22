package controller

import (
	"fmt"
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
		// 返回查询失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to query product data",
		})
		return
	}

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
		// 返回生成令牌失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to generate seckill token",
		})
		return
	}

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
		// 返回秒杀失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Seckill failed",
		})
		return
	}

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
	}

	// 执行模拟支付
	err = g.GoodService.SimulatePayment(orderId, success)
	if err != nil {
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
		// 返回预加载失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to preload goods stock",
		})
		return
	}

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
		// 返回设置失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to set rate limit",
		})
		return
	}

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
	}

	// 获取持续时间参数
	durationStr := c.Query("duration")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 24 * time.Hour // 默认24小时
	}

	// 添加用户到黑名单
	err = g.GoodService.AddToBlacklist(userId, reason, duration)
	if err != nil {
		// 返回添加失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to add user to blacklist",
		})
		return
	}

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
		// 返回获取失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to get blacklist",
		})
		return
	}

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
		// 返回生成令牌失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to generate token",
		})
		return
	}

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
		// 返回令牌无效响应
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Invalid token",
		})
		return
	}

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
		// 返回重置失败响应
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    -1,
			"error":   err.Error(),
			"message": "Failed to reset database",
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Database reset successfully for goods ID: " + goodsIdStr,
	})
}

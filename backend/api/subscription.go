package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"proxy-subscription/models"
	"proxy-subscription/services"

	"github.com/gin-gonic/gin"
)

// GetSubscriptions 获取所有订阅
func GetSubscriptions(c *gin.Context) {
	var subscriptions []models.Subscription
	result := models.DB.Find(&subscriptions)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}

// AddSubscription 添加新订阅
func AddSubscription(c *gin.Context) {
	var subscription models.Subscription
	if err := c.ShouldBindJSON(&subscription); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 处理URL空格
	subscription.URL = strings.TrimSpace(subscription.URL)

	// 验证订阅URL
	if subscription.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订阅URL不能为空"})
		return
	}

	// 设置默认值
	subscription.LastUpdated = time.Now()

	// 保存到数据库
	if err := models.DB.Create(&subscription).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 立即刷新订阅
	if err := services.RefreshSubscription(&subscription); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"subscription": subscription,
			"warning":      "订阅添加成功，但刷新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, subscription)
}

// UpdateSubscription 更新订阅
func UpdateSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var subscription models.Subscription
	if err := models.DB.First(&subscription, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}

	// 绑定请求数据
	if err := c.ShouldBindJSON(&subscription); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 新增URL空格处理
	subscription.URL = strings.TrimSpace(subscription.URL)

	// 更新数据库
	if err := models.DB.Save(&subscription).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// DeleteSubscription 删除订阅
func DeleteSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 删除订阅及其关联的代理节点
	tx := models.DB.Begin()
	if err := tx.Where("subscription_id = ?", id).Delete(&models.Proxy{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Delete(&models.Subscription{}, id).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "订阅已删除"})
}

// RefreshSubscription 刷新订阅
func RefreshSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var subscription models.Subscription
	if err := models.DB.First(&subscription, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}

	// 刷新订阅
	if err := services.RefreshSubscription(&subscription); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "订阅刷新成功", "subscription": subscription})
}

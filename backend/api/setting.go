package api

import (
	"net/http"
	"strconv"

	"proxy-subscription/models"

	"github.com/gin-gonic/gin"
)

// SettingRequest 设置请求结构
type SettingRequest struct {
	AutoRefresh     bool   `json:"autoRefresh"`
	RefreshInterval int    `json:"refreshInterval"`
	DefaultFormat   string `json:"defaultFormat"`
}

// GetSettings 获取所有设置
func GetSettings(c *gin.Context) {
	var settings []models.Setting
	result := models.DB.Find(&settings)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	// 转换为前端友好的格式
	response := SettingRequest{
		AutoRefresh:     false,
		RefreshInterval: 6,
		DefaultFormat:   "base64",
	}

	// 填充实际值
	for _, setting := range settings {
		switch setting.Key {
		case models.SettingAutoRefresh:
			response.AutoRefresh = setting.Value == "true"
		case models.SettingRefreshInterval:
			interval, _ := strconv.Atoi(setting.Value)
			if interval > 0 {
				response.RefreshInterval = interval
			}
		case models.SettingDefaultFormat:
			if setting.Value != "" {
				response.DefaultFormat = setting.Value
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// SaveSettings 保存设置
func SaveSettings(c *gin.Context) {
	var request SettingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx := models.DB.Begin()

	// 保存自动刷新设置
	if err := saveOrUpdateSetting(tx, models.SettingAutoRefresh, strconv.FormatBool(request.AutoRefresh)); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存刷新间隔设置
	if err := saveOrUpdateSetting(tx, models.SettingRefreshInterval, strconv.Itoa(request.RefreshInterval)); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存默认格式设置
	if err := saveOrUpdateSetting(tx, models.SettingDefaultFormat, request.DefaultFormat); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置保存成功"})
}

// saveOrUpdateSetting 保存或更新设置
func saveOrUpdateSetting(tx *gorm.DB, key string, value string) error {
	var setting models.Setting
	result := tx.Where("key = ?", key).First(&setting)

	if result.Error == nil {
		// 更新现有设置
		setting.Value = value
		return tx.Save(&setting).Error
	} else if result.Error == gorm.ErrRecordNotFound {
		// 创建新设置
		setting = models.Setting{Key: key, Value: value}
		return tx.Create(&setting).Error
	} else {
		// 其他错误
		return result.Error
	}
}
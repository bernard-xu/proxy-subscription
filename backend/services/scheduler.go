package services

import (
	"strconv"
	"time"

	"proxy-subscription/models"
	"proxy-subscription/utils"
)

var refreshTicker *time.Ticker
var stopChan chan struct{}

// InitScheduler 初始化定时任务调度器
func InitScheduler() {
	stopChan = make(chan struct{})
	go startScheduler()
	utils.Info("定时任务调度器已启动")
}

// StopScheduler 停止定时任务调度器
func StopScheduler() {
	if refreshTicker != nil {
		refreshTicker.Stop()
	}
	close(stopChan)
	utils.Info("定时任务调度器已停止")
}

// startScheduler 启动定时任务调度器
func startScheduler() {
	// 初始检查设置并启动定时器
	updateScheduler()

	// 每小时检查一次设置变更
	settingCheckTicker := time.NewTicker(1 * time.Hour)
	defer settingCheckTicker.Stop()

	for {
		select {
		case <-settingCheckTicker.C:
			updateScheduler()
		case <-stopChan:
			return
		}
	}
}

// updateScheduler 更新调度器设置
func updateScheduler() {
	// 获取自动刷新设置
	var autoRefreshSetting models.Setting
	result := models.DB.Where("key = ?", models.SettingAutoRefresh).First(&autoRefreshSetting)

	// 如果设置不存在或未启用自动刷新，停止现有的定时器
	if result.Error != nil || autoRefreshSetting.Value != "true" {
		if refreshTicker != nil {
			refreshTicker.Stop()
			refreshTicker = nil
		}
		return
	}

	// 获取刷新间隔设置
	var intervalSetting models.Setting
	result = models.DB.Where("key = ?", models.SettingRefreshInterval).First(&intervalSetting)

	// 默认间隔为6小时
	interval := 6
	if result.Error == nil {
		parsedInterval, err := strconv.Atoi(intervalSetting.Value)
		if err == nil && parsedInterval > 0 {
			interval = parsedInterval
		}
	}

	// 重新设置定时器
	if refreshTicker != nil {
		refreshTicker.Stop()
	}

	refreshTicker = time.NewTicker(time.Duration(interval) * time.Hour)
	go func() {
		for {
			select {
			case <-refreshTicker.C:
				refreshAllEnabledSubscriptions()
			case <-stopChan:
				return
			}
		}
	}()

	utils.Info("自动刷新已启用，间隔: %d小时", interval)
}

// refreshAllEnabledSubscriptions 刷新所有启用的订阅
func refreshAllEnabledSubscriptions() {
	utils.Info("开始自动刷新订阅...")

	// 获取所有启用的订阅
	var subscriptions []models.Subscription
	result := models.DB.Where("enabled = ?", true).Find(&subscriptions)
	if result.Error != nil {
		utils.Error("获取订阅失败: %v", result.Error)
		return
	}

	// 刷新每个订阅
	for _, subscription := range subscriptions {
		utils.Info("正在刷新订阅: %s", subscription.Name)
		if err := RefreshSubscription(&subscription); err != nil {
			utils.Error("刷新订阅 %s 失败: %v", subscription.Name, err)
		} else {
			utils.Info("刷新订阅 %s 成功", subscription.Name)
		}
	}

	utils.Info("自动刷新订阅完成")
}

package services

import (
	"log"
	"strconv"
	"time"

	"proxy-subscription/models"
)

var refreshTicker *time.Ticker
var stopChan chan struct{}

// InitScheduler 初始化定时任务调度器
func InitScheduler() {
	stopChan = make(chan struct{})
	go startScheduler()
	log.Println("定时任务调度器已启动")
}

// StopScheduler 停止定时任务调度器
func StopScheduler() {
	if refreshTicker != nil {
		refreshTicker.Stop()
	}
	close(stopChan)
	log.Println("定时任务调度器已停止")
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

	log.Printf("自动刷新已启用，间隔: %d小时\n", interval)
}

// refreshAllEnabledSubscriptions 刷新所有启用的订阅
func refreshAllEnabledSubscriptions() {
	log.Println("开始自动刷新订阅...")

	// 获取所有启用的订阅
	var subscriptions []models.Subscription
	result := models.DB.Where("enabled = ?", true).Find(&subscriptions)
	if result.Error != nil {
		log.Printf("获取订阅失败: %v\n", result.Error)
		return
	}

	// 刷新每个订阅
	for _, subscription := range subscriptions {
		log.Printf("正在刷新订阅: %s\n", subscription.Name)
		if err := RefreshSubscription(&subscription); err != nil {
			log.Printf("刷新订阅 %s 失败: %v\n", subscription.Name, err)
		} else {
			log.Printf("刷新订阅 %s 成功\n", subscription.Name)
		}
	}

	log.Println("自动刷新订阅完成")
}
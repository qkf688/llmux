package models

import (
	"context"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(ctx context.Context, path string) {
	if err := ensureDBFile(path); err != nil {
		panic(err)
	}
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		panic(err)
	}
	DB = db
	if err := db.AutoMigrate(
		&Provider{},
		&Model{},
		&ModelWithProvider{},
		&ModelTemplateItem{},
		&ChatLog{},
		&ChatIO{},
		&Setting{},
		&HealthCheckLog{},
		&ModelSyncLog{},
	); err != nil {
		panic(err)
	}
	// 兼容性考虑
	if _, err := gorm.G[ModelWithProvider](DB).Where("status IS NULL").Update(ctx, "status", true); err != nil {
		panic(err)
	}
	if _, err := gorm.G[ModelWithProvider](DB).Where("customer_headers IS NULL").Updates(ctx, ModelWithProvider{
		CustomerHeaders: map[string]string{},
	}); err != nil {
		panic(err)
	}
	// 初始化默认设置
	initDefaultSettings(ctx)
	// 初始化优先级字段
	initPriorityField(ctx)
}

// initDefaultSettings 初始化默认设置
func initDefaultSettings(ctx context.Context) {
	defaultSettings := []Setting{
		{Key: SettingKeyStrictCapabilityMatch, Value: "false"},   // 默认关闭严格能力匹配
		{Key: SettingKeyAutoWeightDecay, Value: "false"},         // 默认关闭自动权重衰减
		{Key: SettingKeyAutoWeightDecayDefault, Value: "5"},      // 默认权重值5
		{Key: SettingKeyAutoWeightDecayStep, Value: "1"},         // 默认每次失败减少1
		{Key: SettingKeyAutoSuccessIncrease, Value: "true"},      // 默认开启成功调用自增
		{Key: SettingKeyAutoWeightIncreaseStep, Value: "1"},      // 默认每次成功增加1
		{Key: SettingKeyAutoWeightIncreaseMax, Value: "5"},       // 默认权重上限5
		{Key: SettingKeyAutoPriorityDecay, Value: "false"},       // 默认关闭自动优先级衰减
		{Key: SettingKeyAutoPriorityDecayDefault, Value: "10"},   // 默认优先级值10
		{Key: SettingKeyAutoPriorityDecayStep, Value: "1"},       // 默认每次失败减少1
		{Key: SettingKeyAutoPriorityDecayThreshold, Value: "90"}, // 默认优先级阈值90，达到此值自动禁用
		{Key: SettingKeyAutoPriorityIncreaseStep, Value: "1"},    // 默认每次成功增加1
		{Key: SettingKeyAutoPriorityIncreaseMax, Value: "10"},    // 默认优先级上限10
		{Key: SettingKeyLogRetentionCount, Value: "100"}, // 默认保留100条日志，0表示不限制
		{Key: SettingKeyLogRawRequestResponse, Value: `{"request_headers":false,"request_body":false,"response_headers":false,"response_body":false,"raw_response_body":false}`}, // 默认关闭所有原始日志记录
		{Key: SettingKeyDisableAllLogs, Value: "false"}, // 默认不关闭所有日志记录
		// 健康检测相关默认设置
		{Key: SettingKeyHealthCheckEnabled, Value: "false"},              // 默认关闭健康检测
		{Key: SettingKeyHealthCheckInterval, Value: "60"},                // 默认检测间隔60分钟
		{Key: SettingKeyHealthCheckFailureThreshold, Value: "3"},         // 默认失败3次后禁用
		{Key: SettingKeyHealthCheckFailureDisableEnabled, Value: "true"}, // 默认启用失败自动禁用功能
		{Key: SettingKeyHealthCheckAutoEnable, Value: "false"},           // 默认检测成功不自动启用
		{Key: SettingKeyHealthCheckLogRetentionCount, Value: "100"},      // 默认保留100条健康检测日志，0 表示不限制
		{Key: SettingKeyHealthCheckCountAsSuccess, Value: "true"},        // 默认健康检测成功计入成功调用
		{Key: SettingKeyHealthCheckCountAsFailure, Value: "false"},       // 默认健康检测失败不计入失败调用
		// 性能优化相关默认设置
		{Key: SettingKeyDisablePerformanceTracking, Value: "false"}, // 默认不关闭性能追踪
		{Key: SettingKeyDisableTokenCounting, Value: "false"},       // 默认不关闭 token 统计
		{Key: SettingKeyEnableRequestTrace, Value: "true"},          // 默认启用请求追踪
		{Key: SettingKeyStripResponseHeaders, Value: "false"},       // 默认不移除响应头
		{Key: SettingKeyEnableFormatConversion, Value: "true"},      // 默认启用格式转换
		{Key: SettingKeyAutoPriorityDecayDisableEnabled, Value: "true"}, // 默认启用自动禁用功能
		// 模型同步相关默认设置
		{Key: SettingKeyModelSyncEnabled, Value: "false"},         // 默认关闭自动同步
		{Key: SettingKeyModelSyncInterval, Value: "12"},           // 默认12小时同步一次
		{Key: SettingKeyModelSyncLogRetentionCount, Value: "100"}, // 默认保留100条
		{Key: SettingKeyModelSyncLogRetentionDays, Value: "7"},    // 默认保留7天
		// 模型关联相关默认设置
		{Key: SettingKeyAutoAssociateOnAdd, Value: "false"},    // 默认关闭自动关联
		{Key: SettingKeyAutoCleanOnDelete, Value: "false"},     // 默认关闭自动清理
	}

	for _, setting := range defaultSettings {
		// 如果设置不存在则创建
		count, err := gorm.G[Setting](DB).Where("key = ?", setting.Key).Count(ctx, "id")
		if err != nil {
			panic(err)
		}
		if count == 0 {
			if err := gorm.G[Setting](DB).Create(ctx, &setting); err != nil {
				panic(err)
			}
		}
	}
}

// initPriorityField 初始化优先级字段，为现有记录设置默认优先级
func initPriorityField(ctx context.Context) {
	// 为 priority 为 0 的记录设置默认优先级 10
	if _, err := gorm.G[ModelWithProvider](DB).Where("priority = 0 OR priority IS NULL").Update(ctx, "priority", 10); err != nil {
		panic(err)
	}
}

func ensureDBFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

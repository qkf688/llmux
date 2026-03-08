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
		&VirtualModel{},
		&VirtualModelMapping{},
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
	// 初始化自动关联字段
	initAutoAssociateField(ctx)
	// 迁移旧设置键
	migrateSettingKeys(ctx)
}

// initDefaultSettings 初始化默认设置
func initDefaultSettings(ctx context.Context) {
	defaultSettings := []Setting{
		{Key: SettingKeyStrictCapabilityMatch, Value: "false"},           // 默认关闭严格能力匹配
		{Key: SettingKeyAutoWeightDecay, Value: "false"},                 // 默认关闭自动权重衰减
		{Key: SettingKeyAutoWeightDecayDefault, Value: "5"},              // 默认权重值5
		{Key: SettingKeyAutoWeightDecayStep, Value: "1"},                 // 默认每次失败减少1
		{Key: SettingKeyAutoSuccessIncrease, Value: "true"},              // 默认开启成功调用自增
		{Key: SettingKeyAutoWeightIncreaseStep, Value: "1"},              // 默认每次成功增加1
		{Key: SettingKeyAutoWeightIncreaseMax, Value: "5"},               // 默认权重上限5
		{Key: SettingKeyAutoPriorityDecay, Value: "false"},               // 默认关闭自动优先级衰减
		{Key: SettingKeyAutoPriorityDecayDefault, Value: "10"},           // 默认优先级值10
		{Key: SettingKeyAutoPriorityDecayStep, Value: "1"},               // 默认每次失败减少1
		{Key: SettingKeyAutoPriorityDecayThreshold, Value: "90"},         // 默认优先级阈值90，达到此值自动禁用
		{Key: SettingKeyAutoPriorityIncreaseStep, Value: "1"},            // 默认每次成功增加1
		{Key: SettingKeyAutoPriorityIncreaseMax, Value: "10"},            // 默认优先级上限10
		{Key: SettingKeyConsecutiveFailureThreshold, Value: "3"},         // 默认连续失败3次后禁用
		{Key: SettingKeyConsecutiveFailureDisableEnabled, Value: "true"}, // 默认启用连续失败自动禁用
		{Key: SettingKeyLogRetentionCount, Value: "100"},                 // 默认保留100条日志，0表示不限制
		{Key: SettingKeyLogRawRequestResponse, Value: `{"request_headers":false,"request_body":false,"response_headers":false,"response_body":false,"raw_response_body":false}`}, // 默认关闭所有原始日志记录
		{Key: SettingKeyLogRawRequestResponseErrorsOnly, Value: "false"}, // 默认不启用“仅保留错误日志原始请求响应”
		{Key: SettingKeyDisableAllLogs, Value: "false"},                  // 默认不关闭所有日志记录
		// 健康检测相关默认设置
		{Key: SettingKeyHealthCheckEnabled, Value: "false"},              // 默认关闭健康检测
		{Key: SettingKeyHealthCheckInterval, Value: "60"},                // 默认检测间隔60分钟
		{Key: SettingKeyHealthCheckFailureThreshold, Value: "3"},         // 默认失败3次后禁用
		{Key: SettingKeyHealthCheckFailureDisableEnabled, Value: "true"}, // 默认启用失败自动禁用功能
		{Key: SettingKeyHealthCheckAutoEnable, Value: "false"},           // 默认检测成功不自动启用
		{Key: SettingKeyHealthCheckLogRetentionCount, Value: "100"},      // 默认保留100条健康检测日志，0 表示不限制
		{Key: SettingKeyHealthCheckCountAsSuccess, Value: "true"},        // 默认健康检测成功计入成功调用
		{Key: SettingKeyHealthCheckCountAsFailure, Value: "false"},       // 默认健康检测失败不计入失败调用
		{Key: SettingKeyHealthCheckCheckDisabledOnly, Value: "false"},    // 默认检测所有模型（不只检测停用的）
		// 性能优化相关默认设置
		{Key: SettingKeyDisablePerformanceTracking, Value: "false"},     // 默认不关闭性能追踪
		{Key: SettingKeyDisableTokenCounting, Value: "false"},           // 默认不关闭 token 统计
		{Key: SettingKeyEnableRequestTrace, Value: "true"},              // 默认启用请求追踪
		{Key: SettingKeyStripResponseHeaders, Value: "false"},           // 默认不移除响应头
		{Key: SettingKeyEnableFormatConversion, Value: "true"},          // 默认启用格式转换
		{Key: SettingKeyAutoPriorityDecayDisableEnabled, Value: "true"}, // 默认启用自动禁用功能
		// 模型同步相关默认设置
		{Key: SettingKeyModelSyncEnabled, Value: "false"},         // 默认关闭自动同步
		{Key: SettingKeyModelSyncInterval, Value: "12"},           // 默认12小时同步一次
		{Key: SettingKeyModelSyncLogRetentionCount, Value: "100"}, // 默认保留100条
		{Key: SettingKeyModelSyncLogRetentionDays, Value: "7"},    // 默认保留7天
		{Key: SettingKeyModelSyncFilterRules, Value: `[]`},        // 默认无过滤规则
		// 模板模糊匹配相关默认设置
		{Key: SettingKeyTemplateFuzzyMatchEnabled, Value: "false"},         // 默认关闭模糊匹配
		{Key: SettingKeyTemplateFuzzyMatchSeparators, Value: `[":", "-"]`}, // 默认分隔符
		{Key: SettingKeyTemplateFuzzyMatchSuffixes, Value: `["free"]`},     // 默认后缀
		// 模型关联相关默认设置
		{Key: SettingKeyAutoAssociateOnAdd, Value: "false"},          // 默认关闭自动关联
		{Key: SettingKeyAutoCleanOnDelete, Value: "false"},           // 默认关闭自动清理
		{Key: SettingKeyAutoSaveTemplateOnAssociate, Value: "false"}, // 默认关闭关联时自动保存到模板
		// reasoning_effort 参数映射相关默认设置
		{Key: SettingKeyReasoningEffortMappingEnabled, Value: "true"}, // 默认启用映射
		{Key: SettingKeyReasoningEffortDefaultValue, Value: "low"},    // 默认值为 low
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

// initAutoAssociateField 初始化自动关联字段，为现有模型设置默认值
func initAutoAssociateField(ctx context.Context) {
	// 为 auto_associate 为 NULL 的记录设置默认值 true
	if _, err := gorm.G[Model](DB).Where("auto_associate IS NULL").Update(ctx, "auto_associate", true); err != nil {
		panic(err)
	}
}

// migrateSettingKeys 迁移旧设置键到新设置键
func migrateSettingKeys(ctx context.Context) {
	// 迁移 auto_save_template_on_delete 到 auto_save_template_on_associate
	oldKey := "auto_save_template_on_delete"
	newKey := SettingKeyAutoSaveTemplateOnAssociate

	// 检查旧键是否存在
	oldSetting, err := gorm.G[Setting](DB).Where("key = ?", oldKey).First(ctx)
	if err == nil {
		// 检查新键是否已存在
		count, err := gorm.G[Setting](DB).Where("key = ?", newKey).Count(ctx, "id")
		if err != nil {
			panic(err)
		}

		// 如果新键不存在，则创建新键并复制旧键的值
		if count == 0 {
			newSetting := Setting{
				Key:   newKey,
				Value: oldSetting.Value,
			}
			if err := gorm.G[Setting](DB).Create(ctx, &newSetting); err != nil {
				panic(err)
			}
		}

		// 删除旧键
		if _, err := gorm.G[Setting](DB).Where("key = ?", oldKey).Delete(ctx); err != nil {
			panic(err)
		}
	}
}

var dbPath string

// SetDBPath 设置数据库路径
func SetDBPath(path string) {
	dbPath = path
}

// GetDBPath 获取数据库路径
func GetDBPath() string {
	return dbPath
}

func ensureDBFile(path string) error {
	// 保存数据库路径
	SetDBPath(path)

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

package importexport

import (
	"fmt"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// importEntity 配置导入实体步骤（顺序：providers → models → model_providers → settings）。
// createZero 为 nil 表示只清表不写（settings 半残语义）。
type importEntity struct {
	key           string
	clearSQL      string
	clearFailMsg  string
	insertFailMsg string
	// createZero 在事务中插入一条零值记录（现网半残：不映射 JSON 字段）。
	createZero func(tx *gorm.DB) error
}

// importEntities 注册表：统一 Clear + 按条 Insert 零值，禁止再复制四段循环。
var importEntities = []importEntity{
	{
		key:           "providers",
		clearSQL:      "DELETE FROM providers",
		clearFailMsg:  "清空提供商数据失败",
		insertFailMsg: "导入提供商数据失败",
		createZero: func(tx *gorm.DB) error {
			var provider models.Provider
			return tx.Create(&provider).Error
		},
	},
	{
		key:           "models",
		clearSQL:      "DELETE FROM models",
		clearFailMsg:  "清空模型数据失败",
		insertFailMsg: "导入模型数据失败",
		createZero: func(tx *gorm.DB) error {
			var model models.Model
			return tx.Create(&model).Error
		},
	},
	{
		key:           "model_providers",
		clearSQL:      "DELETE FROM model_with_providers",
		clearFailMsg:  "清空模型-提供商关联数据失败",
		insertFailMsg: "导入模型-提供商关联数据失败",
		createZero: func(tx *gorm.DB) error {
			var modelProvider models.ModelWithProvider
			return tx.Create(&modelProvider).Error
		},
	},
	{
		key:          "settings",
		clearSQL:     "DELETE FROM settings",
		clearFailMsg: "清空系统设置失败",
		// createZero 故意为 nil：现网只清不写
	},
}

// ImportError 导入步骤失败（含对用户可见的中文消息，语义对齐现网）。
type ImportError struct {
	Message string
	Err     error
}

func (e *ImportError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *ImportError) Unwrap() error { return e.Err }

// clearTable 执行实体清空 SQL。
func clearTable(tx *gorm.DB, e importEntity) error {
	if err := tx.Exec(e.clearSQL).Error; err != nil {
		return &ImportError{Message: e.clearFailMsg, Err: err}
	}
	return nil
}

// insertZeroRows 对每个 map 元素创建一条零值行（字段未映射，行为保真）。
// 返回 (落库行数, 跳过行数)：无法映射成 map 的元素计入跳过——此前它会静默丢失且不计入
// 任何计数，表现为「导入成功但少了数据」；只清不写的实体（settings）全部计入跳过。
func insertZeroRows(tx *gorm.DB, e importEntity, rows []interface{}) (int, int, error) {
	if e.createZero == nil {
		return 0, len(rows), nil
	}
	inserted := 0
	skipped := 0
	for _, item := range rows {
		if _, ok := item.(map[string]interface{}); !ok {
			skipped++
			continue
		}
		if err := e.createZero(tx); err != nil {
			return inserted, skipped, &ImportError{Message: e.insertFailMsg, Err: err}
		}
		inserted++
	}
	return inserted, skipped, nil
}

// ImportResult 单类实体的导入结果：按实际处理行数计数。
// Rows 语义按 importEntity 分化：providers/models/model_providers 每条落一行（Imported），
// settings 只清不写（Skipped），与 importer.go 的现网半残语义一致。
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ImportOutcome 导入成功的对外结果（API 层）。前端「导入成功！已导入: 提供商 N 条」依赖它。
type ImportOutcome struct {
	Providers    ImportResult `json:"providers"`
	Models       ImportResult `json:"models"`
	Associations ImportResult `json:"associations"`
	Templates    ImportResult `json:"templates"`
	Settings     ImportResult `json:"settings"`
}

// ImportConfigData 在同一事务 db 上按注册表导入；调用方负责 Begin/Commit/Rollback。
// 返回各实体的计数供 API 回显（templates 不在本注册表内，恒零值）。
func ImportConfigData(tx *gorm.DB, config map[string]interface{}) (ImportOutcome, error) {
	outcome := ImportOutcome{}
	counts := map[string]*ImportResult{
		"providers":       &outcome.Providers,
		"models":          &outcome.Models,
		"model_providers": &outcome.Associations,
		"settings":        &outcome.Settings,
	}
	for _, e := range importEntities {
		rows, ok := config[e.key].([]interface{})
		if !ok {
			continue
		}
		if err := clearTable(tx, e); err != nil {
			return outcome, err
		}
		inserted, skipped, err := insertZeroRows(tx, e, rows)
		if err != nil {
			return outcome, err
		}
		if counter, ok := counts[e.key]; ok {
			counter.Imported = inserted
			counter.Skipped = skipped
		}
	}
	return outcome, nil
}

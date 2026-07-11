package importexport

import (
	"fmt"

	"github.com/atopos31/llmio/models"
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
func insertZeroRows(tx *gorm.DB, e importEntity, rows []interface{}) error {
	if e.createZero == nil {
		return nil
	}
	for _, item := range rows {
		if _, ok := item.(map[string]interface{}); !ok {
			continue
		}
		if err := e.createZero(tx); err != nil {
			return &ImportError{Message: e.insertFailMsg, Err: err}
		}
	}
	return nil
}

// ImportConfigData 在同一事务 db 上按注册表导入；调用方负责 Begin/Commit/Rollback。
func ImportConfigData(tx *gorm.DB, config map[string]interface{}) error {
	for _, e := range importEntities {
		rows, ok := config[e.key].([]interface{})
		if !ok {
			continue
		}
		if err := clearTable(tx, e); err != nil {
			return err
		}
		if err := insertZeroRows(tx, e, rows); err != nil {
			return err
		}
	}
	return nil
}
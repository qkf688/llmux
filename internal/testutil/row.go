package testutil

import (
	"testing"

	"gorm.io/gorm"
)

// NewRow 落一行测试夹具数据并返回带回填主键的行（测试建行样板单源，Stage 35）。
//
// 治的病：日志/夹具测试各自写「构造实体 + db.Create + t.Fatal」三件套，字段口径散在
// 十几个文件里；样板一散，改一次建行语义就要逐点核对。
//
// 契约：
//   - 建行失败即 t.Fatal：夹具行是后续断言的前提，建不全就该停；
//   - 字段由调用方给全——本函数只收口「怎么建」，不定义「建什么」；
//   - 泛型而非 chatlog 专属：本包被 models 等实体包的测试反向引用，import 任何实体
//     包都会成环（models 测试 → testutil → models），纯 gorm 入参才能立得住。
func NewRow[T any](t *testing.T, db *gorm.DB, row T) T {
	t.Helper()
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("testutil: create row: %v", err)
	}
	return row
}

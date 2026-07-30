package testsupport

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/atopos31/llmio/service/settings"
)

// InitTestDB 初始化临时 SQLite，并同步 repository.Default，测试结束自动清理。
func InitTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存旧连接
	repository.SetDefault(repository.New(models.DB))
	settings.SetDefault(settings.NewStore(models.DB))
	t.Cleanup(func() {
		settings.SetDefault(nil)
		repository.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

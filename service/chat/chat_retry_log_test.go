package chat

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
)

// initRetryLogTestDB 构造一个最小 DB：仅 Provider 表 + 模型定义即可。
// helper 只依赖 repos().Provider.ListByIDs，不碰 chat_log 等大表。
func initRetryLogTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存上一个用例的连接
	repository.SetDefault(repository.New(models.DB))
	t.Cleanup(func() {
		repository.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func TestFindModelProviderIDByLog_EmptyProviderNameSkipsMissingProvider(t *testing.T) {
	// 回归 #13.1：候选集含 ProviderID=999（DB 中不存在），log.ProviderName=""。
	// 旧代码 `providerNameByID[999] == "" == log.ProviderName` 会误命中（返回 id,true）。
	// 新代码显式 checked 取值，map 缺失时 continue，期望 (0,false)。
	initRetryLogTestDB(t)

	ctx := context.Background()
	realProvider := &models.Provider{Name: "real", Type: "openai", Config: `{}`}
	if err := models.DB.Create(realProvider).Error; err != nil {
		t.Fatalf("create real provider: %v", err)
	}
	missingProviderID := uint(9999)

	candidates := map[uint]models.ModelWithProvider{
		1: {ProviderID: realProvider.ID, ProviderModel: "model-a"},
		2: {ProviderID: missingProviderID, ProviderModel: "model-a"},
	}

	log := models.ChatLog{ProviderName: "", ProviderModel: "model-a", Status: "error"}

	id, ok := findModelProviderIDByLog(ctx, log, candidates)
	if ok {
		t.Fatalf("expected (0,false) for empty ProviderName + missing ProviderID, got (%d,%v)", id, ok)
	}
}

func TestFindModelProviderIDByLog_ListByIDsErrorContinues(t *testing.T) {
	// 回归 #13.2：ListByIDs 报错时不应阻断整批；helper 应记录错误并继续循环（即使最终结果是 0,false）。
	initRetryLogTestDB(t)

	ctx := context.Background()
	candidates := map[uint]models.ModelWithProvider{
		1: {ProviderID: 1, ProviderModel: "model-a"},
	}
	log := models.ChatLog{ProviderName: "ghost", ProviderModel: "model-a", Status: "error"}

	// 关 DB 触发 ListByIDs error（不 panic 即可）
	sqlDB, err := models.DB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	id, ok := findModelProviderIDByLog(ctx, log, candidates)
	if ok {
		t.Fatalf("expected (0,false) when ListByIDs fails, got (%d,%v)", id, ok)
	}
}

func TestFindModelProviderIDByLog_ExactMatch(t *testing.T) {
	// 正向路径：候选集含匹配项（ProviderID 在 DB 中存在 + name/model 都对得上），期望返回 (id,true)。
	initRetryLogTestDB(t)

	ctx := context.Background()
	provider := &models.Provider{Name: "p1", Type: "openai", Config: `{}`}
	if err := models.DB.Create(provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	candidates := map[uint]models.ModelWithProvider{
		42: {ProviderID: provider.ID, ProviderModel: "gpt-x"},
	}
	log := models.ChatLog{ProviderName: "p1", ProviderModel: "gpt-x", Status: "error"}

	id, ok := findModelProviderIDByLog(ctx, log, candidates)
	if !ok || id != 42 {
		t.Fatalf("expected (42,true), got (%d,%v)", id, ok)
	}
}

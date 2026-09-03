package models

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

// TestPurgeLegacyUpstreamModels 锁定存量 Provider.Config 死键 startup 清理语义：
// upstream_models 键消失、其余键逐字保留、无键行零改写（幂等）、非 JSON 行跳过不炸。
func TestPurgeLegacyUpstreamModels(t *testing.T) {
	db := testChannelDB(t)

	withDeadKey := &Provider{
		Name:   "legacy-dead-key",
		Type:   "openai",
		Config: `{"base_url":"https://api.example.com/v1","upstream_models":["u1","u2"],"custom_models":["c1"]}`,
	}
	if err := db.Create(withDeadKey).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	noDeadKey := &Provider{
		Name:   "clean",
		Type:   "openai",
		Config: `{"base_url":"https://api.example.com/v1"}`,
	}
	if err := db.Create(noDeadKey).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	broken := &Provider{
		Name:   "broken",
		Type:   "openai",
		Config: "not-json{",
	}
	if err := db.Create(broken).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	purgeLegacyUpstreamModels(context.Background(), db)

	wantCleaned := `{"base_url":"https://api.example.com/v1","custom_models":["c1"]}`

	var got Provider
	if err := db.First(&got, withDeadKey.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if gjson.Get(got.Config, "upstream_models").Exists() {
		t.Fatalf("upstream_models should be purged, got %s", got.Config)
	}
	if got.Config != wantCleaned {
		t.Fatalf("other keys should remain verbatim, got %s", got.Config)
	}

	got = Provider{}
	if err := db.First(&got, noDeadKey.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if got.Config != `{"base_url":"https://api.example.com/v1"}` {
		t.Fatalf("config without dead key should be untouched, got %s", got.Config)
	}

	got = Provider{}
	if err := db.First(&got, broken.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if got.Config != "not-json{" {
		t.Fatalf("non-JSON config should be skipped untouched, got %s", got.Config)
	}

	// 幂等：二次运行零改写。
	purgeLegacyUpstreamModels(context.Background(), db)
	got = Provider{}
	if err := db.First(&got, withDeadKey.ID).Error; err != nil {
		t.Fatalf("reload provider: %v", err)
	}
	if got.Config != wantCleaned {
		t.Fatalf("second purge should be a no-op, got %s", got.Config)
	}
}

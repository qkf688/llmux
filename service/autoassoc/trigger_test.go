package autoassoc

import (
	"context"
	"reflect"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// TestDetectProviderWriteEffect 是「哪些 provider 写会动关联表」的唯一定义处：
// 判定规则改了必须改这张表——收窄漏一行就是漏触发（关联该补不补/该清不清），
// 放宽多一行就是把「改个开关也全库重算」的老问题请回来。
func TestDetectProviderWriteEffect(t *testing.T) {
	cases := []struct {
		name   string
		before ProviderWriteState
		after  ProviderWriteState
		want   ProviderWriteEffect
	}{
		{
			name:   "目录新增 → 补建关联",
			before: ProviderWriteState{Catalog: []string{"a"}},
			after:  ProviderWriteState{Catalog: []string{"a", "b"}},
			want:   ProviderWriteEffect{Associate: true},
		},
		{
			name:   "目录减少 → 清理失效关联",
			before: ProviderWriteState{Catalog: []string{"a", "b"}},
			after:  ProviderWriteState{Catalog: []string{"a"}},
			want:   ProviderWriteEffect{Clean: true},
		},
		{
			name:   "目录增删同时发生 → 两个都触发",
			before: ProviderWriteState{Catalog: []string{"a", "b"}},
			after:  ProviderWriteState{Catalog: []string{"b", "c"}},
			want:   ProviderWriteEffect{Associate: true, Clean: true},
		},
		{
			name:   "目录内容相同、顺序不同 → 不触发（快照已排序）",
			before: ProviderWriteState{Catalog: []string{"a", "b"}},
			after:  ProviderWriteState{Catalog: []string{"b", "a"}},
			want:   ProviderWriteEffect{},
		},
		{
			name:   "目录内容相同、含重复 → 不触发（快照已去重）",
			before: ProviderWriteState{Catalog: []string{"a", "a", "b"}},
			after:  ProviderWriteState{Catalog: []string{"b", "a"}},
			want:   ProviderWriteEffect{},
		},
		{
			name:   "两侧都空 → 不触发",
			before: ProviderWriteState{},
			after:  ProviderWriteState{},
			want:   ProviderWriteEffect{},
		},
		{
			name:   "空目录 → 有目录（新建带 custom_models）→ 补建",
			before: ProviderWriteState{},
			after:  ProviderWriteState{Catalog: []string{"x"}},
			want:   ProviderWriteEffect{Associate: true},
		},
		{
			name:   "有目录 → 空目录（清空 custom_models / 分组全删）→ 清理",
			before: ProviderWriteState{Catalog: []string{"x"}},
			after:  ProviderWriteState{},
			want:   ProviderWriteEffect{Clean: true},
		},
		{
			name:   "拉黑 → 不触发（既有关联行留在表里，路由侧另行过滤）",
			before: ProviderWriteState{Catalog: []string{"a"}},
			after:  ProviderWriteState{Catalog: []string{"a"}, Blacklisted: true},
			want:   ProviderWriteEffect{},
		},
		{
			name:   "取消拉黑 → 补建关联（此前被跳过）",
			before: ProviderWriteState{Catalog: []string{"a"}, Blacklisted: true},
			after:  ProviderWriteState{Catalog: []string{"a"}},
			want:   ProviderWriteEffect{Associate: true},
		},
		{
			name:   "拉黑状态不变、目录不变 → 不触发",
			before: ProviderWriteState{Catalog: []string{"a"}, Blacklisted: true},
			after:  ProviderWriteState{Catalog: []string{"a"}, Blacklisted: true},
			want:   ProviderWriteEffect{},
		},
		{
			name:   "目录新增 + 同时拉黑 → 仍要补建（拉黑不抵消新增）",
			before: ProviderWriteState{Catalog: []string{"a"}},
			after:  ProviderWriteState{Catalog: []string{"a", "b"}, Blacklisted: true},
			want:   ProviderWriteEffect{Associate: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectProviderWriteEffect(tc.before, tc.after); got != tc.want {
				t.Fatalf("effect = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestSnapshotProviderWriteState 钉住快照口径：它必须与重算自己读的是同一份目录
// （分组白名单并集 ∪ custom_models），否则「没变」的判定会与重算实际看到的对不上。
func TestSnapshotProviderWriteState(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.Provider.Create(ctx, &models.Provider{
		Name:   "p1",
		Type:   "openai",
		Config: `{"custom_models":["custom-a","custom-b"]}`,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	providers, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil || len(providers) != 1 {
		t.Fatalf("list providers: %v", err)
	}
	// 分组白名单与 custom_models 重叠一个（custom-b）：快照必须去重，否则同一份
	// 目录会被判成「新增了一个」，白白触发一次全库重算。
	seedProviderModels(t, repos, providers[0].ID, "group-a,custom-b")

	svc := newTestService(t, db, staticMatcher{})
	got, err := svc.SnapshotProviderWriteState(ctx, providers[0])
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if want := []string{"custom-a", "custom-b", "group-a"}; !reflect.DeepEqual(got.Catalog, want) {
		t.Fatalf("catalog = %v, want %v", got.Catalog, want)
	}
	if got.Blacklisted {
		t.Fatal("blacklisted should be false when column is NULL")
	}
}

func TestSnapshotProviderWriteState_BlacklistedAndEmptyCatalog(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repos := repository.New(db)

	if err := repos.Provider.Create(ctx, &models.Provider{
		Name:        "p1",
		Type:        "openai",
		Config:      `{}`,
		Blacklisted: boolPtr(true),
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	providers, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil || len(providers) != 1 {
		t.Fatalf("list providers: %v", err)
	}

	svc := newTestService(t, db, staticMatcher{})
	got, err := svc.SnapshotProviderWriteState(ctx, providers[0])
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(got.Catalog) != 0 {
		t.Fatalf("catalog = %v, want empty", got.Catalog)
	}
	if !got.Blacklisted {
		t.Fatal("blacklisted should be true")
	}
}

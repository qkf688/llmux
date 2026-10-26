package autoassoc

import (
	"context"
	"slices"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/modelsync"
)

// ProviderWriteState 一次 provider 写操作前后的状态快照。
//
// 只收「决定关联表内容」的两个输入：
//   - Catalog：这家提供商可提供的上游模型集合（分组白名单并集 ∪ custom_models）；
//   - Blacklisted：是否被拉黑（拉黑期间该提供商不参与自动关联）。
//
// 名称、地址、类型、端点、分组权重与各开关都不进关联表，故不进快照——
// 快照没变，就说明这次写动不到关联。
type ProviderWriteState struct {
	// Catalog 该提供商可提供的上游模型集合（去重且有序）。
	Catalog []string
	// Blacklisted 是否被拉黑。
	Blacklisted bool
}

// SnapshotProviderWriteState 取 provider 此刻的状态快照。
//
// 目录口径必须与重算自身读的是同一份（modelsync.GetProviderModels：分组白名单
// 并集 ∪ config.custom_models）：判定「没变」才可能与重算实际看到的对得上，
// 否则快照说没变、重算却按另一套目录算，收窄就成了漏触发。
func (s *Service) SnapshotProviderWriteState(ctx context.Context, provider models.Provider) (ProviderWriteState, error) {
	catalog, err := modelsync.GetProviderModels(ctx, provider, s.repositories())
	if err != nil {
		return ProviderWriteState{}, err
	}
	// 目录合并侧（mergeProviderModelCatalog）已去重，这里只排序：给 Catalog 一个
	// 确定的顺序，方便测试与日志比对。Detect 本身按集合比对，两者都不影响判定正确性。
	slices.Sort(catalog)
	return ProviderWriteState{
		Catalog:     catalog,
		Blacklisted: provider.Blacklisted != nil && *provider.Blacklisted,
	}, nil
}

// ProviderWriteEffect 描述一次 provider 写对关联表的影响面。
type ProviderWriteEffect struct {
	// Associate 需要补建关联：可提供的上游模型变多了，或这家从拉黑恢复。
	Associate bool
	// Clean 需要清理失效关联：可提供的上游模型变少了，它名下指向这些模型的关联已失效。
	Clean bool
}

// DetectProviderWriteEffect 比对写前后的状态快照，判定这次写要不要重算关联。
//
// 判定口径与「添加时自动关联 / 删除时自动清理」两个设置项的文案一致：
// 提供商可提供的模型增加 → 补建新关联；减少 → 清掉已失效的关联；
// 从拉黑恢复 → 补回此前被跳过的关联。
// 其余改动（改名、改地址、改类型、改端点、改分组权重、动各开关）不动关联表，
// 不触发重算——重算要遍历全库模型 × 全库提供商，一趟不便宜。
//
// 拉黑本身不触发：拉黑只让该提供商退出自动关联候选（既有关联行留在表里，
// 路由侧另行按黑名单过滤），不改表内容。
func DetectProviderWriteEffect(before, after ProviderWriteState) ProviderWriteEffect {
	added, removed := diffCatalog(before.Catalog, after.Catalog)
	return ProviderWriteEffect{
		Associate: added > 0 || (before.Blacklisted && !after.Blacklisted),
		Clean:     removed > 0,
	}
}

// diffCatalog 比较两个集合，返回「新增数」「减少数」（两侧各自去重）。
func diffCatalog(before, after []string) (added, removed int) {
	beforeSet := make(map[string]struct{}, len(before))
	for _, name := range before {
		beforeSet[name] = struct{}{}
	}
	afterSet := make(map[string]struct{}, len(after))
	for _, name := range after {
		afterSet[name] = struct{}{}
		if _, existed := beforeSet[name]; !existed {
			added++
		}
	}
	for _, name := range before {
		if _, exists := afterSet[name]; !exists {
			removed++
		}
	}
	return added, removed
}

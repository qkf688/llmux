package channel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/models"
)

// SelectGroup 按模型白名单 + 价格权重选择分组（设计定案第 4 节第 1 级）：
//
//	白名单过滤（空 = 不限）→ 按 weight 聚合为「权重档」→ 档间加权随机
//	（复用 balancer.WeightedRandom）→ 档内轮询（同权重组间轮询，Selector 状态）。
//
// 档间加权的权重 = 档内**单组**权重而非档总权重：同权重组由轮询打散为等概率，
// 档权重只表达「某权重档的选中概率」——否则 10 个 weight=1 的组会把
// 单个 weight=2 的组挤到几乎不出现，违背价格导向语义。
//
// weight <= 0 的分组不参与：0 是前端新增分组空输入的落值，「未配置」不该
// 以「配了 0 权重」的身份静默进入随机池。
//
// 无分组通过过滤 → ErrNoGroupMatches，由上层（#13 故障转移）判定分组成败。
func SelectGroup(s *Selector, snapshot *Snapshot, modelName string) (models.KeyGroup, error) {
	buckets := groupWeightBuckets(snapshot.Groups, snapshot.Provider.ID, modelName)
	if len(buckets) == 0 {
		return models.KeyGroup{}, fmt.Errorf(
			"%w: provider %d model %q (%d groups)", ErrNoGroupMatches, snapshot.Provider.ID, modelName, len(snapshot.Groups))
	}

	weightItems := make(map[int]int, len(buckets))
	for w := range buckets {
		weightItems[w] = w
	}
	chosenWeight, err := balancer.WeightedRandom(weightItems)
	if err != nil {
		return models.KeyGroup{}, fmt.Errorf("%w: %v", ErrNoGroupMatches, err)
	}

	gs := buckets[*chosenWeight]
	idx, _ := s.nextGroupInRR(snapshot.Provider.ID, *chosenWeight, len(gs))
	return gs[idx], nil
}

// groupWeightBuckets 白名单过滤 + 按 weight 聚成权重档（map[weight][]KeyGroup，
// 档内保持入参顺序）。组顺序稳定是档内轮询取模正确的前提（同一键的候选顺序
// 不应随请求变化，否则轮询会跳跃）。
func groupWeightBuckets(groups []models.KeyGroup, providerID uint, modelName string) map[int][]models.KeyGroup {
	out := make(map[int][]models.KeyGroup)
	for _, g := range groups {
		if g.ProviderID != providerID {
			continue
		}
		if g.Weight <= 0 {
			continue
		}
		if !groupMatchesModel(g, modelName) {
			continue
		}
		out[g.Weight] = append(out[g.Weight], g)
	}
	return out
}

// groupMatchesModel 分组白名单判定：白名单空 = 不限（设计定案「空 = 未同步/不限制」）。
func groupMatchesModel(g models.KeyGroup, modelName string) bool {
	if strings.TrimSpace(g.Models) == "" {
		return true
	}
	for _, m := range strings.Split(g.Models, ",") {
		if strings.TrimSpace(m) == modelName {
			return true
		}
	}
	return false
}

// GroupCandidates 给出需要逐组打上游的候选：**全部**分组，按 ID ASC。用于模型同步、
// 管理端上游浏览（以及后续 #21 健康检查、#24 按凭据测试）的确定性遍历。
//
// 刻意**不**做白名单过滤、也**不**看 weight，与 SelectGroup 的生产选路口径分道：
//   - 白名单（KeyGroup.Models）约束的是「哪些模型可以被送到这个组」，而拉 /models 要问的是
//     「这个组的凭据在上游实际授权了什么」；按白名单过滤会漏掉尚未同步的组——那些组的白名单
//     正是靠这次同步才填上的（先有鸡还是先有蛋）。
//   - weight<=0 只让组不参与生产选路（前端新增分组的空输入落值），它的凭据仍是真 key，
//     同步/浏览漏掉它就永远看不到它授权的上游模型。
//
// Assembler 已按 ID ASC 装配，这里重排是**契约声明**（遍历顺序不能退化成 SQLite 行序），
// 同时返回副本：调用方排序或就地修改都不会污染快照。
func GroupCandidates(snapshot *Snapshot) []models.KeyGroup {
	out := append([]models.KeyGroup(nil), snapshot.Groups...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

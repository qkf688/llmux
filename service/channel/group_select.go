package channel

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/qkf688/llmux/balancer"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

// selectGroupWithPrefer 带钉定偏好选分组：钉定组仍须过与 SelectGroup 完全相同的
// 白名单/权重过滤（复用 groupWeightBuckets），且至少有一条可调度凭据——钉到「已无可
// 调度凭据的死组」会把请求锁死在失败上，违背粘性「失效即静默回退」契约。命中钉定
// 不推进档内轮询指针（钉定流量不得挪动常规流量的分摊位置）；失效则回退 SelectGroup。
//
// 命中钉定时把本组可调度候选集**一次算出带回**（判据单源 usableCredentials），
// 交 selectCredentialWithPrefer 直接用——原实现钉定守卫扫一遍、选凭据再扫一遍、
// miss 回退还第三遍，同请求同组的候选过滤重复算（Stage 30 收口）。未命中/失效
// 返回 nil，由凭据层自行计算；每请求的过滤恰好算一次。
//
// upstreamModel 是**上游模型名**（白名单命名空间，见 groupMatchesModel）。
func selectGroupWithPrefer(s *Selector, snapshot *Snapshot, upstreamModel string, preferID uint, now time.Time) (models.KeyGroup, []models.Credential, error) {
	if preferID != 0 {
		if g, ok := preferredGroup(snapshot, upstreamModel, preferID); ok {
			if usable, err := usableCredentials(snapshot, g.ID, now); err == nil {
				return g, usable, nil
			}
		}
	}
	g, err := SelectGroup(s, snapshot, upstreamModel)
	return g, nil, err
}

// preferredGroup 在同套过滤产物（groupWeightBuckets）里找钉定组：不入桶 = 钉定失效。
func preferredGroup(snapshot *Snapshot, upstreamModel string, preferID uint) (models.KeyGroup, bool) {
	for _, gs := range groupWeightBuckets(snapshot, upstreamModel) {
		for _, g := range gs {
			if g.ID == preferID {
				return g, true
			}
		}
	}
	return models.KeyGroup{}, false
}

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
// upstreamModel 是**上游模型名**（白名单命名空间，见 groupMatchesModel）。
// 无分组通过过滤 → ErrNoGroupMatches，由上层（#13 故障转移）判定分组成败。
func SelectGroup(s *Selector, snapshot *Snapshot, upstreamModel string) (models.KeyGroup, error) {
	buckets := groupWeightBuckets(snapshot, upstreamModel)
	if len(buckets) == 0 {
		return models.KeyGroup{}, fmt.Errorf(
			"%w: provider %d upstream model %q (%d groups)", ErrNoGroupMatches, snapshot.Provider.ID, upstreamModel, len(snapshot.Groups))
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
func groupWeightBuckets(snapshot *Snapshot, upstreamModel string) map[int][]models.KeyGroup {
	out := make(map[int][]models.KeyGroup)
	custom := isCustomModel(snapshot, upstreamModel)
	for _, g := range snapshot.Groups {
		if g.ProviderID != snapshot.Provider.ID {
			continue
		}
		if g.Weight <= 0 {
			continue
		}
		if !groupMatchesModel(g, upstreamModel, custom) {
			continue
		}
		out[g.Weight] = append(out[g.Weight], g)
	}
	return out
}

// isCustomModel 判断 upstreamModel 是否为供应商级自定义模型（Provider.Config 的
// custom_models 清单）。自定义模型是管理员显式声明「该供应商支持此模型」（上游 /models
// 不返回它），2026-09-28 拍板：供应商级声明授权、不受分组白名单约束，全部分组可服务——
// 白名单的本职是「这组 key 实际授权了什么」，上游不返回的模型永远拿不到白名单成员资格，
// 再拿它过滤就把声明过的模型判成不可服务。清单解析单源在 common.ExtractCustomModels。
func isCustomModel(snapshot *Snapshot, upstreamModel string) bool {
	if upstreamModel == "" {
		return false
	}
	for _, id := range common.ExtractCustomModels(snapshot.Provider.Config) {
		if id == upstreamModel {
			return true
		}
	}
	return false
}

// groupMatchesModel 分组可服务判定（白名单空 = 不限，设计定案「空 = 未同步/不限制」；
// custom = 供应商级自定义模型则不受白名单约束，见 isCustomModel）。
//
// 命名空间是**上游模型名**（供应商侧的 model 标识，即 ModelWithProvider.ProviderModel），
// 与写侧同源：modelsync 拉 /models 后把上游模型 ID 原样填进 KeyGroup.Models，autoassoc
// 的组织级目录按「白名单并集 ∪ custom_models」并要求关联的上游名 ∈ 目录，testapi 诊断
// 也传 ProviderModel。传网关侧模型名（Model.Name / 请求里的 model）会让重命名关联
// （ProviderModel ≠ Model.Name）在名单非空后恒不可选——两套名字必须只留一套。
func groupMatchesModel(g models.KeyGroup, upstreamModel string, custom bool) bool {
	if custom {
		return true
	}
	if strings.TrimSpace(g.Models) == "" {
		return true
	}
	for _, m := range strings.Split(g.Models, ",") {
		m = strings.TrimSpace(m)
		if m == "" {
			// 空条目不算成员（与走同一 CSV 格式的 modelsync.splitModelsCSV 一致）：放行它会让
			// 「关联漏填上游名 + 手改白名单留了尾逗号」两处错误相抵，一路走到上游才炸。
			continue
		}
		if m == upstreamModel {
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

// GroupCandidatesForModel 按分组白名单过滤出「可服务 upstreamModel」的候选组，按 ID ASC，
// **不看 weight、不推进任何轮询状态**——管理端连通性测试（testapi）的确定性口径。
//
// 三条分组口径回答的是不同问题，刻意分道（合并任意两条都会让另一条语义失真）：
//   - SelectGroup：生产**会不会**走这个组（加权随机 + 档内轮询 + weight<=0 排除）；
//   - GroupCandidates：哪些组的凭据值得逐个打上游拉目录（同步/浏览/健康检测）——
//     白名单正是这些动作的产物，拿来过滤会陷入先有鸡还是先有蛋；
//   - 本函数：测「这个模型能不能用」时哪些组**有资格回答**。白名单外的组生产不会用它
//     跑这个模型，拿它测出「通过」反而误导（管理端按钮不该给出生产走不到的结论）。
//
// 与 SelectGroup 不同处只看白名单，不排除 weight<=0 的组：权重决定流量分摊而非可用性，
// 把 weight=0 判成「不可用」会让管理员看到「无候选组」而该组 key 其实完好。
// 白名单空 = 不限（与 groupMatchesModel 同一语义）；供应商级自定义模型同样全部分组有资格
// （与 SelectGroup 同一套可服务判定，保证连通测试与生产选路结论一致）。
// upstreamModel 是上游模型名——调用方传 ModelWithProvider.ProviderModel（testapi 即如此），
// 与生产选路口径单源；传网关名会让「测出通过」与「生产走不到」重新分叉。
func GroupCandidatesForModel(snapshot *Snapshot, upstreamModel string) []models.KeyGroup {
	out := make([]models.KeyGroup, 0, len(snapshot.Groups))
	custom := isCustomModel(snapshot, upstreamModel)
	for _, g := range snapshot.Groups {
		if g.ProviderID != snapshot.Provider.ID {
			continue
		}
		if !groupMatchesModel(g, upstreamModel, custom) {
			continue
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

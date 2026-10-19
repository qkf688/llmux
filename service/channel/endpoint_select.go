package channel

import (
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// SelectEndpoint 按入站协议形状选择端点（设计定案第 6 节两级选择的第 1 级）：
//
//   - 透传路径：入站 wire format ∈ 生效协议集合内的端点协议 → 选中该端点
//   - 转换路径：无匹配 → 选中主格式端点（关联可覆盖，未覆盖时 = ProtocolOfType(Provider.Type)
//     派生的默认协议端点，存量迁移保证存在），供协议转换层作为上游出站形状
//
// 生效协议集合与主格式由 ResolveProtocolScope 解析（关联覆盖优先、供应商默认兜底）。端点协议
// 不参与顺序竞争：同一协议一条端点（设计定案「不做同协议多 URL」），匹配到即返回。未知协议
// 端点跳过——配置损坏的端点不参与任何选择，宁可让上层报 ErrEndpointUnavailable，也不按未知
// 协议静默发适配请求。
func SelectEndpoint(snapshot *Snapshot, clientWire consts.WireFormat) (models.Endpoint, error) {
	scope, err := ResolveProtocolScope(snapshot.Provider.Type, snapshot.Endpoints, snapshot.Association)
	if err != nil {
		return models.Endpoint{}, err
	}

	for _, p := range scope.Allowed {
		wf, ok := consts.WireFormatOfProtocol(p)
		if !ok || wf != clientWire {
			continue
		}
		if ep, ok := scope.EndpointFor(snapshot.Endpoints, p); ok {
			return ep, nil
		}
	}

	if ep, ok := scope.EndpointFor(snapshot.Endpoints, scope.Primary); ok {
		return ep, nil
	}
	return models.Endpoint{}, fmt.Errorf(
		"%w: provider %d (type %s), client wire %s, %d endpoints",
		ErrEndpointUnavailable, snapshot.Provider.ID, snapshot.Provider.Type, clientWire, len(snapshot.Endpoints))
}

// SelectPrimaryEndpoint 取「生效主格式」的端点——**确定性**端点口径，供探活/同步/管理端诊断
// 遍历分组时使用。
//
// 与 SelectEndpoint 的差别是刻意的，两条口径不可合并：
//   - SelectEndpoint 服务 chat 请求，第一优先级是「入站 wire format 命中」，且随 Select
//     推进 Selector 的轮询状态；
//   - 本函数不做入站匹配、不碰任何轮询状态：诊断流量必须可复现（配置没动则每次打到同一条
//     链路），也不能让管理员的一次点击挪动真实流量的端点/key 分配。
//
// mp 决定每个调用点「有没有关联可对应」：有关联的诊断（单模型健康检查、连通测试）必须传，
// 否则测的是供应商默认路径而不是该模型真实走的路；号池按凭据测试与供应商级模型同步传 nil
// （它们本就按供应商口径工作）。
//
// endpoints 需按 ID ASC（Assembler 已保证），否则「同协议多条」时退化为首个命中者。
// 返回 bool 而非 error：调用方对「无主协议端点」各有各的文案（同步记 SkipReason、
// 浏览报 no enabled endpoint matching provider type），此处只给事实不给措辞。
func SelectPrimaryEndpoint(providerType string, endpoints []models.Endpoint, mp *models.ModelWithProvider) (models.Endpoint, bool) {
	scope, err := ResolveProtocolScope(providerType, endpoints, mp)
	if err != nil {
		// 保留 bool 返回（调用方各有自己的文案），但**不得静默**：配置损坏（空集合 /
		// 主格式不在集合内）必须留痕，否则调用方只会说「没有可用端点」，把排障引向端点配置。
		assocID := uint(0)
		if mp != nil {
			assocID = mp.ID
		}
		slog.Warn("channel: resolve protocol scope failed",
			"provider_type", providerType, "association_id", assocID, "error", err)
		return models.Endpoint{}, false
	}
	return scope.EndpointFor(endpoints, scope.Primary)
}

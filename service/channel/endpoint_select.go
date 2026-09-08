package channel

import (
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// SelectEndpoint 按入站协议形状选择端点（设计定案第 6 节两级选择的第 1 级）：
//
//   - 透传路径：入站 wire format ∈ enabled 端点协议 → 选中该端点
//   - 转换路径：无匹配 → 选中主协议端点（= Provider.Type 派生的默认协议端点，
//     存量迁移保证存在），供协议转换层作为上游出站形状
//
// 端点协议不参与顺序竞争：同一协议一条端点（设计定案「不做同协议多 URL」），
// 匹配到即返回。未知协议端点（Protocol 不在 consts.Protocol* 集合）跳过——
// 配置损坏的端点不参与任何选择，宁可让上层报 ErrEndpointUnavailable，
// 也不按未知协议静默发适配请求。
func SelectEndpoint(snapshot *Snapshot, clientWire consts.WireFormat) (models.Endpoint, error) {
	for _, ep := range snapshot.Endpoints {
		if !ep.Enabled {
			continue
		}
		wf, ok := consts.WireFormatOfProtocol(consts.Protocol(ep.Protocol))
		if !ok {
			continue
		}
		if wf == clientWire {
			return ep, nil
		}
	}

	primary := consts.ProtocolOfType(snapshot.Provider.Type)
	for _, ep := range snapshot.Endpoints {
		if ep.Protocol == string(primary) && ep.Enabled {
			return ep, nil
		}
	}
	return models.Endpoint{}, fmt.Errorf(
		"%w: provider %d (type %s), client wire %s, %d endpoints",
		ErrEndpointUnavailable, snapshot.Provider.ID, snapshot.Provider.Type, clientWire, len(snapshot.Endpoints))
}

// SelectPrimaryEndpoint 取主协议（= ProtocolOfType(providerType)）的启用端点中 ID
// 最小者——**确定性**端点口径，供探活/同步/管理端诊断遍历分组时使用。
//
// 与 SelectEndpoint 的差别是刻意的，两条口径不可合并：
//   - SelectEndpoint 服务 chat 请求，第一优先级是「入站 wire format 命中」，且随 Select
//     推进 Selector 的轮询状态；
//   - 本函数不做入站匹配、不碰任何轮询状态：诊断流量必须可复现（配置没动则每次打到同一条
//     链路），也不能让管理员的一次点击挪动真实流量的端点/key 分配。
//
// endpoints 需按 ID ASC（Assembler 已保证），否则「最小者」退化为首个命中者。
// 返回 bool 而非 error：调用方对「无主协议端点」各有各的文案（同步记 SkipReason、
// 浏览报 no enabled endpoint matching provider type），此处只给事实不给措辞。
func SelectPrimaryEndpoint(providerType string, endpoints []models.Endpoint) (models.Endpoint, bool) {
	want := string(consts.ProtocolOfType(providerType))
	for _, ep := range endpoints {
		if ep.Enabled && ep.Protocol == want {
			return ep, true
		}
	}
	return models.Endpoint{}, false
}

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

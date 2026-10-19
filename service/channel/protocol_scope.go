package channel

import (
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// protocolOrder 协议固定先后：解析与兜底都按此顺序收敛，保证同一配置每次选出同一条端点
// （map 遍历顺序不可用于选路）。顺序同时是「只设了集合、没设主格式」时的兜底优先级，
// 单源在 consts.AllProtocols（新增协议漏登记会被 consts 的机械门拦住）。
var protocolOrder = consts.AllProtocols()

// ProtocolScope 一次选路的协议范围（关联覆盖的解析结果）。
type ProtocolScope struct {
	// Allowed 允许走的协议，按 protocolOrder 排序；空 = 没有可用协议。
	Allowed []consts.Protocol
	// Primary 需要协议转换时的落点。**继承态下可能不在 Allowed 里**（供应商主协议端点被停用）——
	// 此时端点解析失败、报 ErrEndpointUnavailable，与改动前的行为一致。
	Primary consts.Protocol
	// Explicit 标记该协议来自关联显式勾选。显式勾选可越过供应商的端点启停；
	// 端点缺失时合成空 URL 端点走 base_url（与协议检测探测未配置协议同一做法）。
	Explicit map[consts.Protocol]bool
}

// ResolveProtocolScope 把「供应商端点 + 供应商类型 + 关联覆盖」解析成生效的协议范围。
//
// 规则（关联层默认完整继承供应商那层，覆盖优先）：
//   - 集合：关联未设置 → 供应商已启用端点集；已设置 → 恰好是勾选集（忽略供应商启停）。
//   - 主格式：关联设了 → 用它；未设且集合也未设 → 供应商主格式（= 今天的兜底，行为逐字不变）；
//     只设了集合 → 供应商主格式若在集合内用它，否则集合内固定先后第一条。
//
// 非法覆盖（空集合、未知协议、主格式不在集合内）返回错误而**不静默回落**：回落会让请求
// 走到用户明确排除的协议上，配置错误被伪装成正常选路。
func ResolveProtocolScope(providerType string, endpoints []models.Endpoint, mp *models.ModelWithProvider) (ProtocolScope, error) {
	var overrideSet *[]string
	var overridePrimary *string
	if mp != nil {
		overrideSet = mp.Protocols
		overridePrimary = mp.PrimaryProtocol
	}

	scope := ProtocolScope{Explicit: map[consts.Protocol]bool{}}
	if overrideSet != nil {
		want := make(map[consts.Protocol]bool, len(*overrideSet))
		for _, raw := range *overrideSet {
			p := consts.Protocol(raw)
			if _, ok := consts.TypeOfProtocol(p); !ok {
				return ProtocolScope{}, fmt.Errorf("%w: association %d allows unknown protocol %q",
					ErrEndpointUnavailable, mp.ID, raw)
			}
			want[p] = true
		}
		for _, p := range protocolOrder {
			if want[p] {
				scope.Allowed = append(scope.Allowed, p)
				scope.Explicit[p] = true
			}
		}
		if len(scope.Allowed) == 0 {
			return ProtocolScope{}, fmt.Errorf("%w: association %d allows no protocol", ErrEndpointUnavailable, mp.ID)
		}
	} else {
		enabled := make(map[consts.Protocol]bool, len(endpoints))
		for _, ep := range endpoints {
			if !ep.Enabled {
				continue
			}
			p := consts.Protocol(ep.Protocol)
			if _, ok := consts.TypeOfProtocol(p); !ok {
				continue
			}
			enabled[p] = true
		}
		for _, p := range protocolOrder {
			if enabled[p] {
				scope.Allowed = append(scope.Allowed, p)
			}
		}
	}

	primary := consts.ProtocolOfType(providerType)
	switch {
	case overridePrimary != nil:
		p := consts.Protocol(*overridePrimary)
		if _, ok := consts.TypeOfProtocol(p); !ok {
			return ProtocolScope{}, fmt.Errorf("%w: association %d primary protocol %q is unknown",
				ErrEndpointUnavailable, mp.ID, *overridePrimary)
		}
		if !protocolIn(scope.Allowed, p) {
			return ProtocolScope{}, fmt.Errorf("%w: association %d primary protocol %q not in allowed set %v",
				ErrEndpointUnavailable, mp.ID, p, scope.Allowed)
		}
		primary = p
	case overrideSet != nil && !protocolIn(scope.Allowed, primary):
		// 只覆盖了集合：供应商主格式不在集合内 → 固定先后第一条（集合已保证非空）。
		primary = scope.Allowed[0]
	}
	scope.Primary = primary
	return scope, nil
}

// EndpointOfProtocol 取该协议的端点（快照按 ID ASC，取首个即确定性最小者）。
// includeDisabled=true 时把停用端点也认作"可取"——关联显式勾选与协议检测必须共用这一档：
// 检测打的 URL 必须就是写入后生产流量要打的 URL（否则会出现"检测说可用、落地打到另一条 URL"）。
// false 时只认启用端点（继承态 = 供应商默认，改动前的规则）。
// bool=false 表示没有可取端点，调用方自行决定合成端点还是报不可用。
func EndpointOfProtocol(endpoints []models.Endpoint, p consts.Protocol, includeDisabled bool) (models.Endpoint, bool) {
	for _, ep := range endpoints {
		if ep.Protocol != string(p) {
			continue
		}
		if !ep.Enabled && !includeDisabled {
			continue
		}
		return ep, true
	}
	return models.Endpoint{}, false
}

// EndpointFor 取该协议在本快照里的生效端点。
//   - 显式勾选：供应商配了就用它（**忽略 enabled**——关联覆盖优先于供应商启停）；
//     没配则合成一条空 URL 的启用端点，BuildConfig 回落 Provider base_url。
//   - 继承态：只认已启用端点（今天的规则；同协议有停用行时继续往后找）。
func (s ProtocolScope) EndpointFor(endpoints []models.Endpoint, p consts.Protocol) (models.Endpoint, bool) {
	explicit := s.Explicit[p]
	if ep, ok := EndpointOfProtocol(endpoints, p, explicit); ok {
		return ep, true
	}
	if !explicit {
		return models.Endpoint{}, false
	}
	return models.Endpoint{Protocol: string(p), Enabled: true}, true
}

// protocolIn 判断协议是否在已收敛（protocolOrder 排序）的集合内。
func protocolIn(set []consts.Protocol, p consts.Protocol) bool {
	for _, got := range set {
		if got == p {
			return true
		}
	}
	return false
}

package channel

import (
	"errors"

	"github.com/qkf688/llmux/models"
)

// Snapshot 是某供应商在一次选路上的全部输入快照。
// 选择逻辑只消费本结构（纯逻辑、无 DB 依赖）：端点的启停/协议、分组的
// 白名单/权重/凭据来源、以及按分组归并好的凭据候选（内联 GroupID + 号池
// PoolID 两侧，装配时归并）。由 assemble.go 从 repository 装配，测试可手工构造。
type Snapshot struct {
	// Provider 组织级字段来源（Type 决定主协议端点，Config 含默认 base_url）。
	Provider models.Provider
	// Endpoints 该供应商全部端点（含 disabled；选择时按 Enabled 过滤）。
	Endpoints []models.Endpoint
	// Groups 该供应商全部分组（白名单过滤 + 权重选择）。
	Groups []models.KeyGroup
	// CredentialsByGroup 以分组 ID 为键的凭据候选（装配时已归并内联与号池两侧）。
	CredentialsByGroup map[uint][]models.Credential
}

// SelectionResult 一次选路的完整命中：端点 → 分组 → 凭据 + 组装后的动态 config。
// chat 链路（S3-2）以 Config 作为 providers.New 的参数来源；#13 ChatLog 记录
// 消费 Endpoint/Group/Credential 命中信息（含 UpstreamURL 继承链解析结果）。
type SelectionResult struct {
	Endpoint   models.Endpoint
	Group      models.KeyGroup
	Credential models.Credential
	// UpstreamURL 继承链解析后的实际出站 URL（端点 URL 非空用之，否则 Provider.Config.base_url）。
	UpstreamURL string
	// Config 动态 config JSON：base_url 按继承链覆盖、api_key 为选中凭据明文。
	Config string
}

// 三层选择的分层错误：每层报告「本层无可用候选」，由调用方（chat 链路 /
// #13 故障转移）决定层内转移还是向上失败。sentinel 判定用 errors.Is。
var (
	// ErrEndpointUnavailable 无可用端点：无 enabled 端点，或转换路径的主协议端点缺失/停用。
	ErrEndpointUnavailable = errors.New("channel: no selectable endpoint")
	// ErrNoGroupMatches 没有分组通过白名单过滤（含全部 weight<=0 不参与）。
	ErrNoGroupMatches = errors.New("channel: no group matches")
	// ErrNoCredentialAvailable 选中分组下没有可用凭据（全部 disabled/error/冷却中）。
	ErrNoCredentialAvailable = errors.New("channel: no credential available")
)

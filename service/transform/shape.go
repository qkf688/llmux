package transform

import (
	"bytes"
	"fmt"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
)

// ValidateResponseShape 是「检测类内容门」的形状判定（Stage 19）：上游状态码通过
// （common.IsPassingStatus）之后，再判「这份内容是不是所测 wire format 的合法响应」。
// 为什么在本包：「按协议形状解析响应」是转换层的既有职责（FormatAdapter.ParseResponse
// 的注册地），检测侧只搬运结论，不在别处另写一套解析（DRY）。
//
// 口径四条（门只挡形状不挡语义）：
//  1. 响应体达到诊断读体上限（common.UpstreamDiagnosticReadLimitBytes，1 MiB）判
//     「形状不可验」而非判健康——LimitReader 截断后的 JSON 不可信，「无法验证却判健康」
//     会重新打开假健康面；探针体是短补全，达上限本身即异常。判据随本函数走（Stage 19
//     审查回批：原先散在 service/healthcheck 调用点，与「检测类内容判定只走本函数」矛盾）。
//  2. 空回复判过——合法但 0 content 的响应不算故障，与 RunStreamProbe「空流也算
//     Passed=true」的既有口径一致；
//  3. 非空体必须过 common.JSONContentError——错误形状判据与 chat 提交门共用一份
//     （见 common/ssecontent.go 头注释）。200 + HTML 登录页 / 垃圾页 / 错误信封在这条挡下。
//  4. 必须能被本 wire format 的解析入口解析出响应对象——反代/网关回的「合法 JSON 对象
//     但字段类型全不对」在这里挡下（协议形状含字段类型）。
//
// 与 chat 提交门的口径差（刻意，Stage 19 审查回批定案）：直通链路不解析、能吞下字段怪异
// 的响应，本门判不健康——关联同时服务协议转换链，在那条链上这类响应会解析失败。失败后果是
// 经 handleCheckResult 连败 → 衰减 / 自动停用关联：故要求**连续**失败才到停用阈值，错误
// 文本（parse <format> response: …）可归因，autoEnable 在恢复后重新启用。
// 残余边界（与 chat 提交门既有边界④同口径）：{"code":401,"message":"…"} 这类「合法 JSON
// 对象但语义是错误页」两家都不挡，由调用方 / 客户端自行发现。
//
// 刻意不判 Content-Type 头：网关乱写头很常见，判内容比判头误判率低。
// 未注册的 wire format 先决报错、不回退 OpenAI——与 getAdapter 同一口径（漏配不得静默）；
// 代价（刻意）：type 已注册但漏注册适配器时每轮检查恒报错 → 到阈值自动停用关联，
// 「配置缺漏显式失败」好过静默按错协议测。
func ValidateResponseShape(format consts.WireFormat, body []byte) error {
	adapter, err := getAdapter(format)
	if err != nil {
		return err
	}
	if len(body) >= common.UpstreamDiagnosticReadLimitBytes {
		return fmt.Errorf("response shape unverifiable: body reaches the %d-byte diagnostic read cap (truncated or oversized)", common.UpstreamDiagnosticReadLimitBytes)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := common.JSONContentError(body); err != nil {
		return err
	}
	parsed, err := adapter.ParseResponse(body)
	if err != nil {
		return fmt.Errorf("parse %s response: %w", format, err)
	}
	if parsed == nil {
		return fmt.Errorf("parse %s response: no response object", format)
	}
	return nil
}

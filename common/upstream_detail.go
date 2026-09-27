package common

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// upstreamDetailMaxBytes 诊断细节回显的截断上限：上游错误响应偶有整页 HTML/超长回显，
// 不截断会灌进管理端 JSON（testapi 旧实现无上限，pools 旧实现有——收敛取保守口径）。
const upstreamDetailMaxBytes = 512

// UpstreamDiagnosticReadLimitBytes 探针 / 健康检查 / 连通测试端点读取上游响应体的字节上限（1 MiB）。
// 这些路径读响应体只用于：未通过判定（见 IsPassingStatus）时提取 ≤512B 的人读错误细节，或解析极小的
// completion 探测/测试结果——探测 prompt / strict-schema 天然把合法响应约束到几十 KiB 内，远小于此值。
// Stage 19 起健康检查的成功分支也按本上限读体做内容形状判定（判据在 transform.ValidateResponseShape，
// 超限判「形状不可验」而非判健康）：上限只挡上游异常/被篡改时返回的超大响应整块进内存；
// LimitReader 语义区分不了「被截断」与「恰好等长」，调用方按 len(读取值) == 本值**视为**截断。
const UpstreamDiagnosticReadLimitBytes = 1 << 20

// ReadCappedBody 按字节上限读取响应体，是所有诊断/探针/测试路径读取上游 Body 的单一入口。
// 超过上限的部分被丢弃（LimitReader 语义），读取超时/网络错误仍原样向上返回，与调用方的
// ErrorTypeTimeout 归类逻辑兼容。
func ReadCappedBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, UpstreamDiagnosticReadLimitBytes))
}

// ModelsListReadLimitBytes Models() 拉取上游模型清单的读体上限（Stage 27，此前成功路径
// 无上限整读）：定档 10 MiB 的量级依据——模型清单是上游响应里最小的对象之一，每模型
// ~100-200B（id/object/created/owned_by 四键），现实供应商从几个到几千个模型（openrouter
// 级聚合商数百个 ≈ 100 KiB 量级），10 MiB 容纳数百倍余量；超限只可能是上游异常/被篡改
// （劫持页、垃圾洪水），按解码失败处置。刻意不同于诊断读体的 1 MiB（UpstreamDiagnosticReadLimitBytes）：
// 那是「短补全探针」的上限，模型清单体量天然大一个数量级，沿用 1 MiB 是拍脑袋不是定档。
const ModelsListReadLimitBytes = 10 << 20

// ReadModelsListBody 按清单上限读取模型清单响应体：providers 两个 Models()（成功路径
// 解码前）专用。返回的读取值达上限 = 视为截断（LimitReader 语义区分不了「恰好等长」），
// 调用方按解码失败处置。
func ReadModelsListBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, ModelsListReadLimitBytes))
}

// upstreamUnwrapMaxDepth 错误信封递归剥套的层数上限：网关会把上游 provider 的原始错误体
// JSON 编码成字符串再嵌进自己的 message（opencode go 即此形状），实际观测 2~3 层封顶，
// 该上限只挡病态自嵌套，不构成对真实层数的约束。
const upstreamUnwrapMaxDepth = 5

// ExtractUpstreamErrorDetail 上游错误体的细节提取单一来源：沿 JSON 信封的
// error / message 字段递归追到最内层人读文本（{"error":{"message":…}}、
// {"error":"<内嵌JSON字符串>"}、{"type":…,"message":…} 等形状及其任意嵌套），
// 解析不出则回退原文，最终统一截断。completion 探针（testapi / #24-B）与
// react/structured-output 测试端点共用，消灭「JSON 解析」与「原文截断」两套实现各说各话的漂移。
func ExtractUpstreamErrorDetail(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	detail := ""
	var envelope map[string]any
	if json.Unmarshal(bodyBytes, &envelope) == nil {
		detail = upstreamMessageFromObject(envelope, upstreamUnwrapMaxDepth)
	}
	if detail == "" {
		detail = string(bodyBytes)
	}

	return truncateUpstreamDetail(detail)
}

// UpstreamErrorDetail 未通过结论（见 IsPassingStatus）的完整细节：提取内容 + 状态码行。状态码必须出现在回显
// 文本里（改造前它由 ErrorWithHttpStatus 的 code 承载，收敛进 data.error 后不能丢这个事实）。
func UpstreamErrorDetail(statusCode int, bodyBytes []byte) string {
	return ExtractUpstreamErrorDetail(bodyBytes) + "\nHTTP 状态码: " + strconv.Itoa(statusCode)
}

// upstreamMessageFromObject 从一个 JSON 错误对象追人读文本：
// error 字段是对象→继续向内剥；是字符串→按文本解套；都没有再看本层 message。
func upstreamMessageFromObject(envelope map[string]any, depthBudget int) string {
	if depthBudget <= 0 {
		return ""
	}
	if errField, ok := envelope["error"]; ok {
		switch node := errField.(type) {
		case map[string]any:
			if message := upstreamMessageFromObject(node, depthBudget-1); message != "" {
				return message
			}
		case string:
			if message := upstreamMessageFromText(node, depthBudget-1); message != "" {
				return message
			}
		}
	}
	if message, ok := envelope["message"].(string); ok {
		return upstreamMessageFromText(message, depthBudget-1)
	}
	return ""
}

// upstreamMessageFromText 处理字符串形态的错误载荷：本身是 JSON 对象则递归解套
// （网关把 provider 错误原文塞进 message 的场景），否则它就是人读文本。
func upstreamMessageFromText(text string, depthBudget int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		var nested map[string]any
		if json.Unmarshal([]byte(trimmed), &nested) == nil {
			if message := upstreamMessageFromObject(nested, depthBudget); message != "" {
				return message
			}
		}
	}
	return trimmed
}

func truncateUpstreamDetail(detail string) string {
	// rune 边界回退与截断标记见 common.TruncateUTF8 单源（Stage 2-D 收敛，勿在此另造一套）。
	return TruncateUTF8(detail, upstreamDetailMaxBytes)
}

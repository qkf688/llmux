package chatcore

import (
	"net/http"

	"github.com/qkf688/llmux/common"
)

// BuildHeaders 组装生产转发（/v1 热路径）的出站头。**四层优先级链**（方案 §3，
// 低→高，后写覆盖前写）：
//
//	入站白名单透传（with_header 开时）< key 级头 keyHeaders < 关联级 customHeaders < providers 权威头
//
// 第一层 with_header 的语义是「白名单透传入站头」（common.PassthroughAllowedHeader，
// 与 testapi 诊断链路同一判据，单一数据源）——旧实现 Clone 整份入站头会把终端用户
// Cookie/Referer/Accept-Encoding 发给第三方供应商，且 Accept-Encoding 透传会禁用 Go
// Transport 透明解压、把 gzip 上游误判死（口径调研见
// .local/find-issues/网关热路径chatcore整包透传入站头给上游.md）。
// keyHeaders 是调用方 key 级自定义头（#6，管理员显式通道，不经白名单、与 with_header 正交）。
// customHeaders 是关联级管理员显式通道：同样不设限且后写覆盖（撞名时关联级赢——
// 渠道要求比客户端伪装更具体，方案 §3 定案）。
// 第四层 providers BuildReq 随后以凭据为权威覆写鉴权/Content-Type，是本函数之外的
// 最高优先级。凭据头是死通道且**两家同规则**（4b 收口，单源执法 providers/auth.go）：
// auth_type 选定哪种认证方式就只发哪种凭据头，对侧凭据头在构造点删除——
// Authorization / x-api-key 无论 key 级/关联级怎么配都无法让双头并发送达上游。
func BuildHeaders(source http.Header, withHeader bool, keyHeaders, customHeaders map[string]string, stream bool) http.Header {
	header := http.Header{}
	if withHeader {
		common.MergePassthroughHeaders(header, source)
	}

	if stream {
		header.Set("X-Accel-Buffering", "no")
	}

	for key, value := range keyHeaders {
		header.Set(key, value)
	}

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}

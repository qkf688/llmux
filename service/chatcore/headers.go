package chatcore

import (
	"net/http"

	"github.com/qkf688/llmux/common"
)

// BuildHeaders 组装生产转发（/v1 热路径）的出站头。with_header 的语义是
// 「白名单透传入站头」（common.PassthroughAllowedHeader，与 testapi 诊断链路同一
// 判据，单一数据源）——旧实现 Clone 整份入站头会把终端用户 Cookie/Referer/
// Accept-Encoding 发给第三方供应商，且 Accept-Encoding 透传会禁用 Go Transport
// 透明解压、把 gzip 上游误判死（口径调研见
// .local/find-issues/网关热路径chatcore整包透传入站头给上游.md）。
// customHeaders 是管理员显式通道：不经白名单限制且后写覆盖（Authorization 之类
// 入站被挡的头仍可显式配置送达上游）；providers BuildReq 随后还会以凭据为权威
// 覆写鉴权/Content-Type，出站最终形状不受本函数放行的调试头影响。
func BuildHeaders(source http.Header, withHeader bool, customHeaders map[string]string, stream bool) http.Header {
	header := http.Header{}
	if withHeader {
		common.MergePassthroughHeaders(header, source)
	}

	if stream {
		header.Set("X-Accel-Buffering", "no")
	}

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}

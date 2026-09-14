// 纯逻辑：白名单透传判据与合流入站头。被 service/chatcore（生产转发）与
// handler/testapi（诊断测试）共用（AGENTS 4.4 跨域纯逻辑下沉），单一数据源——
// 两条链路若各持一份名单必然漂移。
//
// 口径来源：.local/find-issues/网关热路径chatcore整包透传入站头给上游.md 的同行调研
// （new-api 白名单派交叉验证）。入站透传 ⊥ 管理员自定义头：customer_headers 不经本
// 判据，显式配置永远可盖一切。
package common

import (
	"net/http"
	"net/textproto"
	"strings"
)

// PassthroughAllowedHeader 判定入站请求头能否在 with_header=true 时透传给第三方供应商。
// 默认拒绝（白名单），放行的是无会话/来源泄露风险的调试性头：X- 前缀自定义头（中转渠道
// 的 org ID、beta 开关等多走这里）、UA 与协议 beta/版本头。排除项各有实因：
// Authorization/Cookie/X-Api-Key＝会话与网关凭据；Referer/Origin＝内部地址结构；
// Accept-Encoding/Connection/Proxy-*/Host＝传输层头，自设会禁用 Go Transport 透明解压，
// gzip 上游会被读流侧误判死；Sec-*＝浏览器握手内部头。
// X- 前缀不是可信分区（IETF 早已放弃其语义），反代写入的 IP/URI 变体逐名挡掉。
// 注意：Anthropic-Beta/Version 两项对 anthropic 供应商实为死通道——providers/anthropic.go
// 的 BuildReq 以供应商配置为权威无条件覆写/清除；放行它们只为不无谓地丢头（openai 式
// 端点仍消费 Openai-Beta），统一「透传 ⊂ 出站候选」的口径。
// key 必须先经 textproto.CanonicalMIMEHeaderKey 规范化再进来比较。
func PassthroughAllowedHeader(canonicalKey string) bool {
	switch canonicalKey {
	case "User-Agent", "Openai-Beta", "Anthropic-Beta", "Anthropic-Version":
		return true
	}
	switch {
	case strings.HasPrefix(canonicalKey, "X-Forwarded-"),
		canonicalKey == "X-Real-Ip",
		canonicalKey == "X-Client-Ip",
		canonicalKey == "X-True-Ip",
		canonicalKey == "X-Remote-Addr",
		canonicalKey == "X-Original-Uri":
		return false // 客户端 IP / 内部路由信息外泄给第三方，无调试价值
	}
	return strings.HasPrefix(canonicalKey, "X-") && canonicalKey != "X-Api-Key"
}

// MergePassthroughHeaders 把 src 中通过白名单判据的入站头按名合入 dst（同名多值追加），
// 其余键级缺席——dst 里既有的值不受影响（不剥 dst 已有头，出站权威由调用链后置 Set 保证）。
// src 的 map 键可能是非规范大小写（http.Header 不保证调用方构造方式），先规范化再判定，
// 合入 dst 的键一律规范形。
func MergePassthroughHeaders(dst, src http.Header) {
	for key, values := range src {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if PassthroughAllowedHeader(canonical) {
			dst[canonical] = append(dst[canonical], values...)
		}
	}
}

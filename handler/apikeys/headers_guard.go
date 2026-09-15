package apikeys

import (
	"fmt"
	"net/textproto"
	"strings"
)

// sanitizeOutboundHeaders 是 key 级出站头的写入闸（Create/PATCH 共用）：
// ① 名必须是 RFC 7230 token 并规范化为 MIME Title-Case（与 Go client 出站 Set 的
// 规范化同口径，存储/回显/发送三者不再两套样子）；② 规范化后撞名直接拒绝——
// Go map 迭代序随机，裸 API 提交「x-app + X-App」时装配层 Set 顺序不确定，
// 「后者赢」只在有序输入里成立，对 map 输入唯一确定的语义是拒绝；
// ③ 值拒控制字符（对齐 net/http/httpguts 的出站判据）——非法值入库会让
// Client.Do 以 invalid header field value 失败，落入网络失败分支误伤无关
// 供应商的冷却/权重，管理员笔误的代价被放大成全局故障。
// 这是语法关不是头名黑名单：Cookie/Authorization 等合法名照放（方案 §2 裁决）。
func sanitizeOutboundHeaders(headers map[string]string) (map[string]string, error) {
	if headers == nil {
		return nil, nil
	}
	sanitized := make(map[string]string, len(headers))
	for name, value := range headers {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return nil, fmt.Errorf("header name is empty")
		}
		if !isHTTPToken(trimmed) {
			return nil, fmt.Errorf("header name %q contains characters not allowed in RFC 7230 token", name)
		}
		if !isValidHeaderValue(value) {
			return nil, fmt.Errorf("header %q value contains control characters (CR/LF injection or transport-invalid)", name)
		}
		canonical := textproto.CanonicalMIMEHeaderKey(trimmed)
		if _, exists := sanitized[canonical]; exists {
			return nil, fmt.Errorf("duplicate header names after case normalization: %q", canonical)
		}
		sanitized[canonical] = value
	}
	return sanitized, nil
}

// isHTTPToken 判 RFC 7230 tchar 集（与 httpguts 同表：token 字符 = alnum + "!#$%&'*+-.^_`|~"）。
func isHTTPToken(s string) bool {
	const tokenSpecials = "!#$%&'*+-.^_`|~"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		default:
			if !strings.ContainsRune(tokenSpecials, rune(c)) {
				return false
			}
		}
	}
	return len(s) > 0
}

// isValidHeaderValue 对齐 net/http/httpguts.ValidHeaderFieldValue：允许 HTAB、
// 0x20-0x7E、0x80-0xFF（Go transport 自身的出站判据，非 ASCII 上游侧自行处理）；
// 拒 CR/LF/NUL 与其他控制字符。
func isValidHeaderValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '\t' {
			continue
		}
		if c < ' ' || c == 0x7f {
			return false
		}
	}
	return true
}

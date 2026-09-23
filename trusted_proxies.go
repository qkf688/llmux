package main

import "strings"

// parseTrustedProxies 解析 TRUSTED_PROXIES（逗号分隔的 IP / CIDR，如
// "10.0.0.0/8,192.168.1.1"），语义是「引擎可以采信其 X-Forwarded-For 的代理网段」。
//
// 空输入恒返回非 nil 空切片，显式表达「未配置 = 全关」：gin 的**默认**（从不调用
// SetTrustedProxies）是 0.0.0.0/0 + ::/0 信任全部代理，必须显式调用才收窄；
// SetTrustedProxies(nil) 与 SetTrustedProxies([]string{}) 在 gin v1.10 同为全关
// （prepareTrustedCIDRs 对 nil 返回空集、isTrustedProxy 对空集判 false）。
// 合法性校验交给 SetTrustedProxies（单一判据），非法值在那里 fail fast，不做静默回退。
func parseTrustedProxies(raw string) []string {
	proxies := []string{}
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			proxies = append(proxies, v)
		}
	}
	return proxies
}

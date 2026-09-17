package providers

import (
	"net/http"
)

// 认证方式配置值词表跨 provider 共用。Stage 4b 裁决：认证方式是凭据头的唯一权威——
// auth 头与对侧凭据头在供应商构造请求处（所有头合并之后）一并定夺，入站透传 / key 级 /
// 关联级自定义头都不可能让两个凭据头并发送达上游。双头并送时「上游最终认哪个」由对方
// 实现定义（官方与各中转口径不一，本仓库不假设任何一家的具体行为），网关只承诺确定行为；
// 要打通只认另一种凭据头的中转，改选本供应商的认证方式，而不是往自定义头里塞。
// config 键按协议拆分（审查 P0 裁决）：anthropic 读 auth_type（存量键，语义不变）、
// openai 系读 openai_auth_type——混合协议供应商同一份 config 喂两家 adapter，共键会让
// 一家的方言静默翻转另一家已上线端点的凭据头。

const (
	AuthTypeBearer  = "bearer"
	AuthTypeXAPIKey = "x-api-key"
)

// applyAuthHeader 按 authType 写凭据头并删除对侧凭据头。
// authType 为空或不是已知枚举时回退 defaultAuth（anthropic=x-api-key、
// openai 式=bearer——保持两家的历史默认，存量配置零迁移）。
// 位点是唯一执法点：所有 provider 出站路径（转发 BuildReq、模型列表 Models）
// 都必须走这里，禁止再手写 Set("Authorization")。
func applyAuthHeader(h http.Header, authType, defaultAuth, apiKey string) {
	switch authType {
	case AuthTypeBearer, AuthTypeXAPIKey:
	default:
		authType = defaultAuth
	}
	if authType == AuthTypeXAPIKey {
		h.Del("Authorization")
		h.Set("x-api-key", apiKey)
		return
	}
	h.Del("x-api-key")
	h.Set("Authorization", "Bearer "+apiKey)
}

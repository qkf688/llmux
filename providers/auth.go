package providers

import (
	"encoding/json"
	"net/http"

	"github.com/qkf688/llmux/models"
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

// DeriveConfigAuthType 从 config JSON 派生认证方式**展示值**（Stage 20 派生化）：
// 按**主类型**方言键派生——anthropic 读 auth_type、其余（openai 系）读 openai_auth_type，
// 与各 adapter 反序列化字段同键（anthropic.go / openai_base.go，见上拆键动因）。
// ⚠️ 混合协议行（如 type=openai 且勾 anthropic 端点）的对侧端点读的是另一个键，
// 顶层展示槽只表达主类型方言，不承诺覆盖对侧端点的实际出站头（顶层只有一种可表达）。
// 值原样返回、不做值域过滤：垃圾值原样暴露比静默回默认更利于发现配置错误
// （出站仍由 applyAuthHeader 兜底回类型默认）。键缺失/空/坏 JSON = nil（未设置）。
// 顶层 models.Provider.AuthType 即由本函数派生（gorm:"-" 不落库，消灭双写）。
func DeriveConfigAuthType(providerType, configJSON string) *string {
	var probe struct {
		AuthType       string `json:"auth_type"`
		OpenAIAuthType string `json:"openai_auth_type"`
	}
	if err := json.Unmarshal([]byte(configJSON), &probe); err != nil {
		return nil
	}
	v := probe.OpenAIAuthType
	if providerType == TypeAnthropic {
		v = probe.AuthType
	}
	if v == "" {
		return nil
	}
	return &v
}

// FillDerivedAuthType 填充 models.Provider 的派生展示字段（Stage 20）：AuthType 不落库
// （gorm:"-"），真值单源在 config。读侧出库后统一调用（详情/列表/导出共用同一填充
// 动作，避免各域自写赋值行漂移；providers→models 是合法依赖方向）。
func FillDerivedAuthType(p *models.Provider) {
	p.AuthType = DeriveConfigAuthType(p.Type, p.Config)
}

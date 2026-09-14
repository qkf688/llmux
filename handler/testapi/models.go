package testapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

// ProviderModelTestRequest 是 POST /api/providers/:id/test 的请求体。
//
// GroupID = 0（或键缺失）＝自动口径：由后端按「白名单命中的候选组 → ID ASC 首个有可用
// 凭据的组」解析；>0 ＝锁定该分组（管理端 C2 形态下点某个分组徽标＝只测这一条链路），
// 显式锁组时天然不做白名单过滤（用户已指定测谁，白名单是生产路由的事）。
type ProviderModelTestRequest struct {
	Model   string `json:"model"`
	GroupID uint   `json:"group_id"`
}

// testLink 是一次诊断测试解析出的完整链路与测试对象：端点 → 分组 → 凭据三层都定死，
// Client 里已注入凭据明文。
//
// 为什么必须由本文件统一产出、四个测试端点不得各自组装客户端：S6 起 Provider.Config
// 不再承载 api_key（密钥只存在于 credentials 表，运行时由 channel.BuildConfig 注入明文），
// 任何直用 Provider.Config 打上游的路径都恒被上游判 401（#19 修的就是这四处）。
type testLink struct {
	// Client 已装配好的上游客户端（channel.NewClientFromSelection 产物）。
	Client providers.Provider
	// Type 客户端实际注册类型——按**端点协议**派生而非 Provider.Type，与 chat 主链路
	// 同源（service/chat/chat_retry_loop.go 亦按 selection.Endpoint.Protocol 取类型）。
	Type string
	// ProviderName 供应商名（日志与错误上下文用）。
	ProviderName string
	// Model 打上游的模型名（关联级 = ModelWithProvider.ProviderModel；供应商级 = 请求体 model）。
	Model string
	// Group 实际命中的分组。测试结果必须能归位到组：否则管理员看到 ✕ 时无从判断
	// 是哪条链路的 key 坏了，也无法按组复测。
	Group models.KeyGroup
	// WithHeader / CustomerHeaders 仅关联级有值（供应商级为 nil）：入站头透传开关与
	// 该关联的自定义头。
	WithHeader      *bool
	CustomerHeaders map[string]string
}

// errInvalidID ID 非数字。与 handler 既有的 "Invalid ID format" 措辞对应
// （改造前非数字 ID 会被 GORM 判成 ErrRecordNotFound 而回 404，语义不准）。
var errInvalidID = errors.New("testapi: invalid id")

// errNoWhitelistedGroup 关联级口径：模型不在任何分组白名单内。用 sentinel 而非裸文案，
// handler 据此把 error_type 归到 validation（区别于凭据问题的 auth、配置的 provider）。
var errNoWhitelistedGroup = errors.New("testapi: no group whitelisted for model")

// errInvalidGroup 请求体显式指定的 group_id 不属于该供应商（多半是弹窗里的分组列表已
// 过期——供应商全量更新会重建分组并换 ID）。属于调用方输入错误，走 400 而非一次失败的测试。
var errInvalidGroup = errors.New("testapi: group does not belong to provider")

// ResolveAssociationLink 解析「关联 → 供应商 → 主协议端点 → 白名单候选组 → 首个可用
// 凭据」这条诊断链路，id 为 ModelWithProvider 主键（GET /api/test/:id、
// /test/react/:id、/test/structured-output/:id 共用）。
//
// 关联级按白名单过滤（channel.GroupCandidatesForModel）：这三处测的是「这条关联能不能
// 服务这个模型」，而生产路由只会用白名单命中的组，拿一个生产走不到的组测出「通过」
// 等于给出假信号。健康检查（service/healthcheck）刻意不过滤——它失败的代价是禁用关联，
// 宁可宽松也不能误禁，两条口径的差别记在 channel.GroupCandidatesForModel 的注释里。
func ResolveAssociationLink(ctx context.Context, id string) (*testLink, error) {
	mpID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", errInvalidID, id)
	}
	mp, err := repos().ModelWithProvider.Get(ctx, uint(mpID))
	if err != nil {
		return nil, err
	}
	provider, err := repos().Provider.Get(ctx, mp.ProviderID)
	if err != nil {
		return nil, err
	}
	snapshot, err := channel.NewAssembler(repos()).Assemble(ctx, *provider)
	if err != nil {
		return nil, err
	}

	groups := channel.GroupCandidates(snapshot)
	if len(groups) == 0 {
		return nil, fmt.Errorf("供应商 %q 未配置任何凭据分组", provider.Name)
	}
	candidates := channel.GroupCandidatesForModel(snapshot, mp.ProviderModel)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: 模型 %q 不在任何分组白名单内，生产路由同样不会选中它",
			errNoWhitelistedGroup, mp.ProviderModel)
	}

	return buildLink(snapshot, mp.ProviderModel, candidates, mp.WithHeader, mp.CustomerHeaders)
}

// ResolveProviderLink 解析供应商级测试链路（POST /api/providers/:id/test），model 为
// 上游目录里的任意模型名。
//
// 与关联级的两处差别是刻意的：
//   - **不按白名单过滤**：本端点测的是「这个模型在这个供应商上实际能不能用」，而
//     KeyGroup.Models 是上次同步的快照，未同步/未回填的组会被误判成不可用；
//   - groupID > 0 时锁定该组（候选只剩它），供管理端按分组徽标逐条验证。
func ResolveProviderLink(ctx context.Context, id string, model string, groupID uint) (*testLink, error) {
	providerID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", errInvalidID, id)
	}
	provider, err := repos().Provider.Get(ctx, uint(providerID))
	if err != nil {
		return nil, err
	}
	snapshot, err := channel.NewAssembler(repos()).Assemble(ctx, *provider)
	if err != nil {
		return nil, err
	}

	var candidates []models.KeyGroup
	if groupID > 0 {
		group, ok := groupOfProvider(snapshot, groupID)
		if !ok {
			return nil, fmt.Errorf("%w: 分组 %d 不属于供应商 %q（供应商更新后分组 ID 会变，请重新拉取分组列表）",
				errInvalidGroup, groupID, provider.Name)
		}
		candidates = []models.KeyGroup{group}
	} else {
		candidates = channel.GroupCandidates(snapshot)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("供应商 %q 未配置任何凭据分组", provider.Name)
	}

	return buildLink(snapshot, model, candidates, nil, nil)
}

// buildLink 在候选组里挑出**第一条凭据可用**的链路，装配一次、只发一次上游请求。
//
// 为什么跨组只挑「凭据可用」而不做「上游失败换下一组」：① 本包的测试体是真实计费
// 的 completion（providers/meta_bodies.go），供应商级批量测试并发 3，失败放大成 N 倍
// 花费不可接受；② react 是 SSE 流式多轮，重跑会向客户端重复推事件；③ 测试失败无副
// 作用（不像健康检查会禁用关联），报「首个候选组失败」后管理员可按组徽标复测定位。
// 代价已知：组内只验首个可用凭据，排在后面的坏 key 测不出来（那是 credprobe 与 #24 的职责）。
func buildLink(snapshot *channel.Snapshot, model string, candidates []models.KeyGroup, withHeader *bool, customHeaders map[string]string) (*testLink, error) {
	endpoint, ok := channel.SelectPrimaryEndpoint(snapshot.Provider.Type, snapshot.Endpoints)
	if !ok {
		return nil, fmt.Errorf("供应商 %q 无启用的 %s 协议端点", snapshot.Provider.Name, consts.ProtocolOfType(snapshot.Provider.Type))
	}
	providerType, ok := consts.TypeOfProtocol(consts.Protocol(endpoint.Protocol))
	if !ok {
		// 防御性分支（不变量）：SelectPrimaryEndpoint 只按 ProtocolOfType 的取值匹配端点，
		// 该协议必在 consts.protocolMetaTable 内；真出现说明两张表发散，宁失败不猜测。
		return nil, fmt.Errorf("端点协议 %q 无法派生供应商类型（端点 %d 配置损坏）", endpoint.Protocol, endpoint.ID)
	}

	now := time.Now()
	skipped := make([]string, 0, len(candidates))
	for _, group := range candidates {
		probe, err := channel.ProbeGroup(snapshot, group, endpoint, now)
		if err != nil {
			// 仅「本组无可用凭据」继续换组；解密失败/协议未知等全局问题直接上抛，
			// 换下一个组必然同样失败，静默吞掉只会把配置问题伪装成凭据问题。
			if !errors.Is(err, channel.ErrNoCredentialAvailable) {
				return nil, err
			}
			skipped = append(skipped, fmt.Sprintf("分组 %q: %s", group.Name, err))
			continue
		}
		// 掩码凭据只进日志不进响应（C2 定案：✕ 不铺开错误详情，UI 只到分组一层，
		// 「用的哪把 key」只剩这一行日志可查）。明文由 ProbeGroup 收在包内。
		slog.Info("testapi 诊断链路已装配",
			"provider", snapshot.Provider.Name,
			"model", model,
			"endpoint_id", probe.EndpointID,
			"protocol", probe.Protocol,
			"upstream_url", probe.UpstreamURL,
			"group", probe.Group.Name,
			"credential_id", probe.CredentialID,
			"key_masked", probe.KeyMasked,
		)
		return &testLink{
			Client:          probe.Client,
			Type:            providerType,
			ProviderName:    snapshot.Provider.Name,
			Model:           model,
			Group:           group,
			WithHeader:      withHeader,
			CustomerHeaders: customHeaders,
		}, nil
	}

	// 措辞分单组/多组两档：单候选组＝显式锁定分组（前端按组徽标点测）或只有一个组，
	// 直接说「该分组无可用凭据」才是可执行的结论；多组时把逐组原因留下供排查。
	if len(candidates) == 1 {
		return nil, fmt.Errorf("%w: 分组 %q 无可用凭据（已停用或冷却中）", channel.ErrNoCredentialAvailable, candidates[0].Name)
	}
	return nil, fmt.Errorf("%w: %d 个候选分组均无可用凭据（%s）",
		channel.ErrNoCredentialAvailable, len(candidates), strings.Join(skipped, "; "))
}

// groupOfProvider 在快照分组里按 ID 取组（快照已由 Assembler 限定本供应商）。
func groupOfProvider(snapshot *channel.Snapshot, groupID uint) (models.KeyGroup, bool) {
	for _, g := range snapshot.Groups {
		if g.ID == groupID {
			return g, true
		}
	}
	return models.KeyGroup{}, false
}

// linkErrorType 把链路解析失败映射到既有 error_type 值域（webui/src/lib/errors.ts 的
// ERROR_TYPES 镜像同一集合）：凭据问题 → auth、白名单未命中 → validation、
// 其余（无端点/无分组/装配或解密失败）→ provider（供应商侧配置不可用）。
func linkErrorType(err error) string {
	switch {
	case errors.Is(err, channel.ErrNoCredentialAvailable):
		return "auth"
	case errors.Is(err, errNoWhitelistedGroup):
		return "validation"
	default:
		return "provider"
	}
}

// passthroughAllowedHeader 判定入站请求头能否在 with_header=true 时透传给第三方供应商。
// 默认拒绝（白名单），放行的是无会话/来源泄露风险的调试性头：X- 前缀自定义头（中转渠道
// 的 org ID、beta 开关等多走这里）、UA 与协议 beta/版本头。排除项各有实因：
// Authorization/Cookie/X-Api-Key＝管理端会话与网关凭据；Referer/Origin＝内部地址结构；
// Accept-Encoding/Connection/Proxy-*/Host＝传输层头，自设会禁用 Go Transport 透明解压，
// gzip 上游会被探针误判死；Sec-*＝浏览器握手内部头。
// X- 前缀不是可信分区（IETF 早已放弃其语义），反代写入的 IP/URI 变体逐名挡掉。
// 注意：Anthropic-Beta/Version 两项对 anthropic 供应商实为死通道——providers/anthropic.go
// 的 BuildReq 以供应商配置为权威无条件覆写/清除；放行它们只为不无谓地丢头（openai 式
// 端点仍消费 Openai-Beta），统一「透传 ⊂ 出站候选」的口径。
// key 必须先经 textproto.CanonicalMIMEHeaderKey 规范化再进来比较。
func passthroughAllowedHeader(canonicalKey string) bool {
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

// BuildTestHeaders 组装诊断测试的出站头。with_header 的语义是「白名单透传入站头」
// 而非整包转发（旧实现 Clone 整份入站头，会把管理员 JWT/Cookie 发给供应商）；
// customer_headers 是管理员显式配置的通道，不设白名单限制且后写覆盖透传结果。
func BuildTestHeaders(source http.Header, withHeader *bool, customHeaders map[string]string) http.Header {
	header := http.Header{}

	if withHeader != nil && *withHeader {
		for key, values := range source {
			canonical := textproto.CanonicalMIMEHeaderKey(key)
			if passthroughAllowedHeader(canonical) {
				header[canonical] = append(header[canonical], values...)
			}
		}
	}

	for key, value := range customHeaders {
		header.Set(key, value)
	}

	return header
}

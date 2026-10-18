package testapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
	"github.com/qkf688/llmux/service/protocolprobe"
)

// 本文件是「协议支持检测」的**链路解析**：把「关联 + 供应商 + 分组 + 凭据」落成
// 三条协议各自的目标（客户端 / 测试体 / 请求 URL）。
//
// 与既有四个测试端点不同构的地方只有两处、且都是必需的：① 白名单候选组**只挑一个**
// （同组同凭据——换 key 会让三条协议的结论掺进凭据差异，结论就废了）；② 按协议分别
// 装配客户端（端点 URL 逐协议解析：已配端点用其 URL，未配则按「继承 Base URL」合成
// 空 URL 端点，由 channel.BuildConfig 走既有继承链解析，本文件不另抄一份 URL 拼接）。
//
// 探测编排（两段并发、三条探针）在 service/protocolprobe；本文件不打任何上游请求。
// 全程只读：不写任何配置、不改端点启停、不碰关联状态。

// urlSourceInherited / urlSourceEndpoint 说明该协议用的 URL 从哪来（展示用，判定不用）。
const (
	urlSourceInherited = "inherited" // 未配端点，按「继承 Base URL」合成端点
	urlSourceEndpoint  = "endpoint"  // 已配该协议端点，用它的 URL
)

// protocolOrder 探测顺序固定为 OpenAI → Responses → Anthropic：回包数组顺序与前端展示
// 顺序、测试断言顺序三者必须一致，故显式写死而不依赖 map 遍历。
var protocolOrder = []consts.Protocol{consts.ProtocolOpenAI, consts.ProtocolResponses, consts.ProtocolAnthropic}

// protocolTargetSet 一次检测的全部目标与链路上下文。
type protocolTargetSet struct {
	ProviderName string
	Model        string
	Group        models.KeyGroup
	MainProtocol consts.Protocol
	Targets      []protocolprobe.Target
	Header       http.Header
}

// ResolveProtocolTargets 解析检测链路并为三条协议各装配一个客户端。
func ResolveProtocolTargets(ctx context.Context, id string, source http.Header) (*protocolTargetSet, error) {
	mp, provider, snapshot, err := resolveAssociationSnapshot(ctx, id)
	if err != nil {
		return nil, err
	}

	// 白名单口径与既有测试端点一致（channel.GroupCandidatesForModel 的注释解释了为什么
	// 这里要过滤：生产路由只会用命中的组）。
	candidates := channel.GroupCandidatesForModel(snapshot, mp.ProviderModel)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: 模型 %q 不在任何分组白名单、也不在自定义模型清单内，生产路由同样不会选中它（可将该模型加入供应商的自定义模型，或等模型同步回填白名单）",
			errNoWhitelistedGroup, mp.ProviderModel)
	}

	group, cred, err := firstUsableGroup(snapshot, candidates)
	if err != nil {
		return nil, err
	}

	set := &protocolTargetSet{
		ProviderName: provider.Name,
		Model:        mp.ProviderModel,
		Group:        group,
		MainProtocol: consts.ProtocolOfType(provider.Type),
		// 出站头沿用既有测试端点的同一份组装（入站头白名单透传 + 该关联的自定义头）：
		// 透传开关没配的话，管理员为真实流量配的鉴权头不会带进这次诊断，白测一次。
		Header: BuildTestHeaders(source, mp.WithHeader, mp.CustomerHeaders),
	}
	for _, p := range protocolOrder {
		target, err := buildProtocolTarget(ctx, snapshot.Provider, snapshot.Endpoints, cred, p, mp.ProviderModel)
		if err != nil {
			return nil, err
		}
		set.Targets = append(set.Targets, target)
	}
	return set, nil
}

// buildProtocolTarget 为一条协议装配客户端、测试体与请求 URL。
func buildProtocolTarget(ctx context.Context, provider models.Provider, endpoints []models.Endpoint, cred models.Credential, protocol consts.Protocol, model string) (protocolprobe.Target, error) {
	providerType, ok := consts.TypeOfProtocol(protocol)
	if !ok {
		return protocolprobe.Target{}, fmt.Errorf("端点协议 %q 无法派生供应商类型（consts 表配置损坏）", protocol)
	}
	format, ok := consts.WireFormatOfProtocol(protocol)
	if !ok {
		return protocolprobe.Target{}, fmt.Errorf("端点协议 %q 无法派生响应体形状（consts 表配置损坏）", protocol)
	}
	meta, ok := providers.MetadataOf(providerType)
	if !ok || len(meta.TestBody) == 0 {
		return protocolprobe.Target{}, fmt.Errorf("供应商类型 %q 未注册连通性测试体，无法探测该协议", providerType)
	}

	endpoint, configured := endpointOfProtocol(endpoints, protocol)
	client, err := channel.NewClientFromSelection(provider, endpoint, cred)
	if err != nil {
		return protocolprobe.Target{}, err
	}
	source := urlSourceInherited
	if configured {
		source = urlSourceEndpoint
	}
	target := protocolprobe.Target{
		Protocol:           protocol,
		ProviderType:       providerType,
		Format:             format,
		Body:               append([]byte(nil), meta.TestBody...),
		URLSource:          source,
		EndpointConfigured: configured,
		Client:             client,
	}
	// 请求 URL 取自一次不发网的 BuildReq：拼装路径（chat/completions / responses / messages）
	// 只有 providers 一处知道，抄一份映射过来必然随协议路径变更而漂移。
	req, err := client.BuildReq(ctx, http.Header{}, model, target.Body)
	if err != nil {
		return protocolprobe.Target{}, fmt.Errorf("装配 %s 请求 URL 失败: %w", protocol, err)
	}
	target.RequestURL = req.URL.String()
	return target, nil
}

// endpointOfProtocol 取该协议已启用的端点（快照按 ID ASC，故取首个即确定性最小者）；
// 没有则合成一条空 URL 的启用端点——channel.BuildConfig 会按继承链回落到 Provider
// 的 Base URL，与「在供应商表单里勾上这个协议、URL 留空继承」完全等价。
func endpointOfProtocol(endpoints []models.Endpoint, protocol consts.Protocol) (models.Endpoint, bool) {
	for _, ep := range endpoints {
		if ep.Enabled && ep.Protocol == string(protocol) {
			return ep, true
		}
	}
	return models.Endpoint{Protocol: string(protocol), Enabled: true}, false
}

// firstUsableGroup 在候选组里取第一个有可用凭据的组（与 buildLink 的挑组口径一致：
// 只换到「凭据可用」为止，不做「上游失败换下一组」——一次检测要打 9 条请求，
// 逐组重试的花费不可接受）。
func firstUsableGroup(snapshot *channel.Snapshot, candidates []models.KeyGroup) (models.KeyGroup, models.Credential, error) {
	now := time.Now()
	skipped := make([]string, 0, len(candidates))
	for _, group := range candidates {
		cred, err := channel.FirstUsableCredential(snapshot, group.ID, now)
		if err == nil {
			return group, cred, nil
		}
		if !errors.Is(err, channel.ErrNoCredentialAvailable) {
			return models.KeyGroup{}, models.Credential{}, err
		}
		skipped = append(skipped, fmt.Sprintf("分组 %q: %s", group.Name, err))
	}
	if len(candidates) == 1 {
		return models.KeyGroup{}, models.Credential{}, fmt.Errorf("%w: 分组 %q 无可用凭据（已停用或冷却中）",
			channel.ErrNoCredentialAvailable, candidates[0].Name)
	}
	return models.KeyGroup{}, models.Credential{}, fmt.Errorf("%w: %d 个候选分组均无可用凭据（%s）",
		channel.ErrNoCredentialAvailable, len(candidates), strings.Join(skipped, "; "))
}

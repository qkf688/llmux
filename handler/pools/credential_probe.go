package pools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
	"gorm.io/gorm"
)

// completion 探针超时：真实补全含上游生成 + 排队，比 /models 轻探（credprobe 的 5s 口径）宽；
// 且刻意让 ctx deadline（credProbeTimeout）**早于** transport 超时（+ credProbeClientBuffer），
// 使「慢上游」稳定命中 context.DeadlineExceeded → 归类 timeout，而非 transport 的
// "awaiting response headers"（非 DeadlineExceeded）被误归 network。
const (
	credProbeTimeout      = 15 * time.Second
	credProbeClientBuffer = 1 * time.Second
)

// probeMaxDetailBytes 非 2xx 时回显的上游原文截断上限（只报不改、诊断用，防超长响应灌进
// 前端；C2 定案失败只显红，正文主要给服务端日志留证）。
const probeMaxDetailBytes = 512

// errPoolNoReferencingGroup 号池未被任何分组引用：无法从「池 → 分组 → 供应商 → 端点」
// 这条链定位上游，用 sentinel 与真正的内部错误区分（前者是一次失败的测试结论 →
// error_type=provider，后者是 500）。
var errPoolNoReferencingGroup = errors.New("pool is not referenced by any key group")

// TestCredential 号池详情「按凭据测试」（#24-B）：用**指定这一条**凭据打一次真实最小
// completion 探活，回 passed 结论。**只报不改**——绝不写回 Credential.Status/FailCount/Cooldown。
//
// POST /api/pools/:id/credentials/:credId/test
//
// 为什么用 completion 而非 /models 轻探（B 方案）：completion 才证明「这把 key 在这模型上
// 真能跑」，且走 BuildReq+Do 能拿结构化 res.StatusCode 精确归 error_type，修掉了旧 /models
// 版靠错误文本 substring 判 401 的退化。代价（计费）可控：B 是单条点测，花费只在管理员
// 手点一条时产生，不是整池扫描。
//
// 模型来自号池级 test_model（#24-B 迁移：2026-09-10 用户拍板从分组迁到池，配置与点测同屏）；
// 未配置直接判 validation 失败、不打上游——不回退硬编码默认模型，否则未授权模型的 404
// 会被误判成 key 坏，与 #24 目的相反。
//
// 为什么不能复用 channel.ProbeGroup / credprobe.TryRecover：二者内部都是
// FirstUsableCredential（组内首个可用），语义是「这个组活不活」，用它测单条坏 key 会
// 静默打到组里另一把好 key 上、把坏 key 误判成健康（#24 的存在意义恰是逐 key 健康）。
// 装配点必须用 channel.NewClientFromSelection（唯一「喂指定凭据」的入口）。
//
// 上游端点取「引用该池的分组里 ProviderID/GroupID 最小者」的主协议端点，并在响应回显
// 命中的 group——绝大多数池只被一个分组用，歧义罕见；真有歧义时回显让管理员看得见测的
// 哪条链路，不被误导（首版不做端点选择器 / 不做批量测整池，均为刻意收敛）。
func TestCredential(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	credID, ok := httpx.ParseUintParamAllowZero(c, "credId")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	// 越池 / 池不存在 / 凭据不存在 → 404（复用统一收口，零上游请求，不暴露凭据存在性）。
	// 池实体随校验带出：测试模型是池级配置，直接从这里读，不再二次查库。
	cred, pool, ok := getCredentialInPool(c, ctx, poolID, credID)
	if !ok {
		return
	}

	link, err := resolveCredentialTestLink(ctx, poolID)
	if err != nil {
		respondLinkFailure(c, link, err)
		return
	}

	testModel := strings.TrimSpace(pool.TestModel)
	if testModel == "" {
		// Q4（迁移后口径）：号池未配置 test_model → 一次失败的测试结论
		// （error_type=validation），不打上游、不装配 client。硬编码默认模型未必被这把 key
		// 授权，拿它去测会把「配置缺失」伪装成「key 坏」（404/无权限），故显式失败而非回退。
		slog.Info("credential test skipped: pool has no test model configured",
			"pool_id", poolID, "credential_id", credID,
			"provider", link.provider.Name, "group", link.group.Name)
		httpresp.Success(c, failedProbeResult(link, "", common.ErrorTypeValidation,
			"该号池未配置测试模型，无法按凭据测试"))
		return
	}

	client, err := channel.NewClientFromSelection(link.provider, link.endpoint, *cred)
	if err != nil {
		// 装配含解密：失败是配置/密文问题（加密未配置、密文损坏），链路本身没问题，
		// 归 provider（供应商侧不可用），不谎报成一次凭据鉴权失败。
		respondLinkFailure(c, link, err)
		return
	}

	probeBody, ok := credentialProbeBody(consts.Protocol(link.endpoint.Protocol))
	if !ok {
		// 端点协议未知 / 该 provider type 未在 RegisterMetadata 登记 ProbeBody（新供应商漏配）：
		// 属配置态，归 provider，不拿空 body 去打上游谎测。
		respondProbeFailure(c, link, testModel, common.ErrorTypeProvider,
			"该协议未注册 completion 探针体", string(link.endpoint.Protocol))
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx, credProbeTimeout)
	defer cancel()

	req, err := client.BuildReq(probeCtx, nil, testModel, probeBody)
	if err != nil {
		respondProbeFailure(c, link, testModel, common.ErrorTypeNetwork, "构建探针请求失败", err.Error())
		return
	}

	httpClient := providers.GetClientWithProxy(credProbeTimeout+credProbeClientBuffer, client.GetProxy())
	res, err := httpClient.Do(req)
	if err != nil {
		// Do 失败没有状态码：区分超时与其他网络错误（与 testapi「network/timeout 由调用点
		// 判定」同一约定）。
		errType := common.ErrorTypeNetwork
		if errors.Is(err, context.DeadlineExceeded) {
			errType = common.ErrorTypeTimeout
		}
		slog.Warn("credential test probe connect failed",
			"pool_id", poolID, "credential_id", credID,
			"provider", link.provider.Name, "group", link.group.Name, "error", err)
		respondProbeFailure(c, link, testModel, errType, "连接上游失败", err.Error())
		return
	}
	defer res.Body.Close()

	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		respondProbeFailure(c, link, testModel, common.ErrorTypeNetwork, "读取上游响应失败", readErr.Error())
		return
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		errType := common.ErrorTypeFromStatus(res.StatusCode)
		slog.Warn("credential test probe failed",
			"pool_id", poolID, "credential_id", credID,
			"provider", link.provider.Name, "group", link.group.Name,
			"status", res.StatusCode, "error_type", errType)
		respondProbeFailure(c, link, testModel, errType, "上游返回错误", truncateDetail(bodyBytes))
		return
	}

	httpresp.Success(c, CredentialTestResult{
		Passed:     true,
		GroupID:    link.group.ID,
		GroupName:  link.group.Name,
		ProviderID: link.provider.ID,
		Model:      testModel,
	})
}

// credentialTestLink 按凭据测试解析出的上游链路：分组 → 供应商 → 主协议端点三层定死。
// 只承载「链路定位」被回显的字段——测试模型属号池（由调用方从 getCredentialInPool
// 带出的 pool 读取），不塞进本结构，链路是链路、配置是配置，两族数据不同源。
// 不在此装配 client（明文字段留到 NewClientFromSelection 内部），链路解析失败时也能
// 回显「测到哪一步」，与 testapi 的 testLink 同一设计意图。
type credentialTestLink struct {
	group    models.KeyGroup
	provider models.Provider
	endpoint models.Endpoint
}

// resolveCredentialTestLink 从号池反查上游链路：ListByPoolIDs → 按 (ProviderID, GroupID)
// 升序取首个分组 → Provider.Get → Assemble → SelectPrimaryEndpoint（主协议启用端点）。
// 池存在性不在此校验——调用方经 getCredentialInPool（内部 requirePool）已 404 挡过。
//
// 「取最小者」不是「取 ListByPoolIDs 的首行」：后者只按 GroupID 升序，跨供应商时 ProviderID
// 无序，直接 [0] 会随分组创建顺序漂移。显式按 (ProviderID, GroupID) 排序才有确定性口径
// （配置没动则每次打到同一链路，管理员复测可比）。
//
// 无引用分组 → errPoolNoReferencingGroup（池存在但配不出链路，是一次失败的测试结论）。
func resolveCredentialTestLink(ctx context.Context, poolID uint) (*credentialTestLink, error) {
	groups, err := repos().KeyGroup.ListByPoolIDs(ctx, []uint{poolID})
	if err != nil {
		return nil, fmt.Errorf("list referencing groups: %w", err)
	}
	if len(groups) == 0 {
		return nil, errPoolNoReferencingGroup
	}
	sorted := append([]models.KeyGroup(nil), groups...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ProviderID != sorted[j].ProviderID {
			return sorted[i].ProviderID < sorted[j].ProviderID
		}
		return sorted[i].ID < sorted[j].ID
	})
	group := sorted[0]

	provider, err := repos().Provider.Get(ctx, group.ProviderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 供应商已被删但分组残留（DeleteProvider 不级联删分组）：同样是配置态结论。
			return &credentialTestLink{group: group}, errPoolNoReferencingGroup
		}
		return &credentialTestLink{group: group}, err
	}

	snapshot, err := channel.NewAssembler(repos()).Assemble(ctx, *provider)
	if err != nil {
		return &credentialTestLink{group: group, provider: *provider}, err
	}
	endpoint, ok := channel.SelectPrimaryEndpoint(provider.Type, snapshot.Endpoints)
	if !ok {
		return &credentialTestLink{group: group, provider: *provider},
			fmt.Errorf("供应商 %q 无启用的 %s 协议端点", provider.Name, consts.ProtocolOfType(provider.Type))
	}
	return &credentialTestLink{group: group, provider: *provider, endpoint: endpoint}, nil
}

// credentialProbeBody 按端点协议取该 provider type 注册的「最小 completion 探针体」
// （providers.Metadata.ProbeBody，#24-B）。不在 handler 写协议 switch——wire 形状属 providers
// 适配域，新供应商 type 只需在 RegisterMetadata 登记 ProbeBody（OCP，与 TestBody/HealthCheckBody
// 同源）。model 不含在内，由 provider.BuildReq 用 sjson 注入指定 testModel。协议未知或该 type
// 未注册探针体时回 (nil,false)，调用方判失败而不拿空 body 去上游谎测。
func credentialProbeBody(protocol consts.Protocol) ([]byte, bool) {
	providerType, ok := consts.TypeOfProtocol(protocol)
	if !ok {
		return nil, false
	}
	meta, ok := providers.MetadataOf(providerType)
	if !ok || len(meta.ProbeBody) == 0 {
		return nil, false
	}
	return meta.ProbeBody, true
}

// respondLinkFailure 回「链路配不出 / 装配失败」这类测试结论（恒 code 200 + passed:false +
// error_type=provider）：无引用组、供应商已删、无启用主协议端点、解密/装配失败都归此类——
// 它们描述的是「这条凭据当前没有可测的上游链路」，不是一次凭据鉴权/探活失败。
//
// 恒回 200 而非 500/400：前端 apiRequest 在 code!=200 时只抛 message、丢 data，用错误信封
// 回传结论等于前端读不到归位的 group/provider；且这是配置态结论，不是调用方入参错误
// （入参错误如越池已由 getCredentialInPool 回 404）。回显已解析到的 group/provider（可能为
// 0/空串 = 链路更早步失败），与 testapi 的 testResultBase 同一「键恒在」约定。
func respondLinkFailure(c *gin.Context, link *credentialTestLink, err error) {
	httpresp.Success(c, failedProbeResult(link, "", common.ErrorTypeProvider, err.Error()))
}

// respondProbeFailure 回「已选定链路、completion 打到上游后失败」的结论（网络/超时/auth/
// provider/validation）：恒 code 200 + passed:false，回显命中的 group/provider 与所用 model。
// 与 respondLinkFailure 区分：后者是链路本身配不出（恒 provider），前者是链路可用但这一条
// key/上游不通过。
func respondProbeFailure(c *gin.Context, link *credentialTestLink, testModel, errorType, summary, detail string) {
	httpresp.Success(c, failedProbeResult(link, testModel, errorType, fmt.Sprintf("%s: %s", summary, detail)))
}

// failedProbeResult 组装失败态 CredentialTestResult（含 group/provider 回显与所用 model）。
func failedProbeResult(link *credentialTestLink, testModel, errorType, msg string) CredentialTestResult {
	res := CredentialTestResult{ErrorType: errorType, Error: msg, Model: testModel}
	if link != nil {
		res.GroupID = link.group.ID
		res.GroupName = link.group.Name
		res.ProviderID = link.provider.ID
	}
	return res
}

// truncateDetail 截断上游原文到诊断够用长度，避免超长响应灌进前端/日志。
func truncateDetail(b []byte) string {
	if len(b) <= probeMaxDetailBytes {
		return string(b)
	}
	return string(b[:probeMaxDetailBytes]) + "…(truncated)"
}

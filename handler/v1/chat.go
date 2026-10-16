package v1

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service"
	"gorm.io/gorm"
)

// newReqMeta 组装请求元数据：头/IP/UA 之外还把鉴权中间件放下的 CallerKey
// 快照翻译成 models 侧标量（models 不 import middleware，翻译只在此一处）。
// JWT 管理端路径 CallerKey 恒 nil → 归因空态，是区分两类入口的现成判据。
func newReqMeta(c *gin.Context) models.ReqMeta {
	meta := models.ReqMeta{
		Header:    c.Request.Header,
		RemoteIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	// rid 快照（Stage 17）：middleware.RequestID 保证非空且已带来源前缀；
	// 裸 Context 单测直连时为空串，落库侧语义同「无编号」。
	meta.RequestID = c.GetString(consts.ContextKeyRequestID)
	if caller := middleware.CurrentCallerKey(c); caller != nil {
		// 拷值不取 &caller.ID：快照定格进 ReqMeta，不与 gin context 内共享对象
		// 绑同一块内存（#6 给 CallerKey 加字段后该不变量依然成立）。
		callerID := caller.ID
		meta.ApiKeyID = &callerID
		meta.ApiKeyName = caller.Name
		// headers 已由 AuthAPIKey 浅拷贝定格，此处传引用不再拷第二层（快照链上
		// 唯一可写点在装配前不存在：BuildHeaders 只读源 map）。
		meta.Headers = caller.Headers
	}
	return meta
}

// ModelsHandler 列出当前可用模型，直接从数据库读取基础信息并按 OpenAI 协议返回。
func ModelsHandler(c *gin.Context) {
	// 获取真实模型
	llmModels, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	modelsList := make([]providers.Model, 0)
	for _, llmModel := range llmModels {
		modelsList = append(modelsList, providers.Model{
			ID:      llmModel.Name,
			Object:  "model",
			Created: llmModel.CreatedAt.Unix(),
			OwnedBy: "llmux",
		})
	}

	// 获取虚拟模型
	virtualModels, err := gorm.G[models.VirtualModel](models.DB).Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	// 添加虚拟模型到列表
	for _, virtualModel := range virtualModels {
		modelsList = append(modelsList, providers.Model{
			ID:      virtualModel.Name,
			Object:  "model",
			Created: virtualModel.CreatedAt.Unix(),
			OwnedBy: "llmux-virtual",
		})
	}

	httpresp.SuccessRaw(c, providers.ModelList{
		Object: "list",
		Data:   modelsList,
	})
}

func ChatCompletionsHandler(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleOpenAI)
}

func ResponsesHandler(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleOpenAIRes)
}

func Messages(c *gin.Context) {
	chatHandlerByStyle(c, consts.StyleAnthropic)
}

// CountTokens 是 /v1/count_tokens 的占位 handler，返回 501 Not Implemented。
// 该端点仅挂鉴权中间件，待实现真实 token 计数逻辑时替换。
func CountTokens(c *gin.Context) {
	httpresp.ErrorWithHttpStatus(c, http.StatusNotImplemented, http.StatusNotImplemented, "count_tokens not implemented")
}

// chatHandlerByStyle 以入站 style 为入口分发。参数用 consts.Style 收窄取值域；
// 下游（注册表 / chat 编排）仍按裸 string 传递，故在此做唯一一次转换
// （Style 端到端换成 consts.Style 是独立重构，见 service/chat/chat_attempt_request.go 的 TODO）。
func chatHandlerByStyle(c *gin.Context, style consts.Style) {
	beforer, err := service.GetBeforer(string(style))
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	processer, err := service.GetProcesser(string(style))
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	chatHandler(c, beforer, processer, string(style))
}

func chatHandler(c *gin.Context, preProcessor service.Beforer, postProcessor service.Processer, style string) {
	// 网关总超时落请求 ctx（Stage 33）：到点整个请求（含流式输出）被掐断，且超时
	// （deadline）与调用方撤走（取消）在 ctx 上可区分，收尾归因以此为单源
	// （chat_settlement.requestAbandoned）。cancel 在 handler 返回（响应写完）时才
	// 释放——过早 cancel 会掐断正在输出的流。计时从进来到收完，覆盖读体/预处理/
	// 选路/上游调用/流式回写全程。
	ctx, cancelTimeout := service.WithRequestTotalTimeout(c.Request.Context())
	defer cancelTimeout()

	// 读取原始请求体：多读 1 字节探测超限（入站硬上限 consts.MaxInboundBodyBytes，
	// /v1 由调用方 key 鉴权、攻击者可控，Stage 2 资源防御）。
	reqBody, err := io.ReadAll(io.LimitReader(c.Request.Body, consts.MaxInboundBodyBytes+1))
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}
	c.Request.Body.Close()
	if int64(len(reqBody)) > consts.MaxInboundBodyBytes {
		// 413 信封与既有 /v1 错误出口同形状（ErrorWithHttpStatus），不另造协议错误体；
		// 选 413 而非 400：与「JSON 写坏了」区分原因，且 SDK 不会对 413 自动重试。
		httpresp.ErrorWithHttpStatus(c, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge,
			"request body too large (limit 20 MiB)")
		return
	}
	// 预处理、提取模型参数
	before, err := preProcessor(reqBody)
	if err != nil {
		httpresp.ErrorWithHttpStatus(c, http.StatusBadRequest, http.StatusBadRequest, err.Error())
		return
	}
	// 按模型获取可用 provider（reqMeta 先于解析构造：模型不存在/虚拟选模失败的
	// 错误日志也在这一步落库，归因快照必须已在场）
	reqMeta := newReqMeta(c)
	providersWithMeta, err := service.ProvidersWithMetaBymodelsName(ctx, style, *before, reqMeta)
	if err != nil {
		httpresp.InternalServerError(c, err)
		return
	}

	startReq := time.Now()
	// 调用负载均衡后的 provider 并转发
	balanced, err := service.BalanceChat(ctx, service.BalanceInput{
		Start:             startReq,
		Style:             style,
		Before:            *before,
		ProvidersWithMeta: *providersWithMeta,
		ReqMeta:           reqMeta,
	})
	if err != nil {
		var statusCoder interface{ StatusCode() int }
		if errors.As(err, &statusCoder) {
			status := statusCoder.StatusCode()
			httpresp.ErrorWithHttpStatus(c, status, status, err.Error())
			return
		}
		httpresp.InternalServerError(c, err)
		return
	}
	res := balanced.Response
	defer res.Body.Close()

	// 后处理与记录 tokens 走 bgtask 登记（Stage 30，AGENTS §6：会写库的后台 goroutine
	// 必须进登记本，禁裸 go）：关机时由 bgtask.Shutdown 排空（先于关库，main shutdown
	// 四步序），「关机时正在写的请求记录不丢」的职责单属登记本——原 WaitGroup 丝线
	// （handler 自等 + Shutdown 等在途请求的两层接力）随之退场，双轨并存会两套等待互相等。
	// 断流错误路径 return 后同理由排空覆盖：失败留痕（updateChatLogByID）写完才关库。
	// 登记被丢弃（关机收尾段的丢弃窗口）时的放手语义见 attachLogRecord。
	body, finishLog := attachLogRecord(ctx, service.RecordLogInput{
		ReqStart:                   startReq,
		Processer:                  postProcessor,
		LogID:                      balanced.LogID,
		Before:                     *before,
		IOLog:                      providersWithMeta.IOLog,
		ProviderName:               balanced.ProviderName,
		SideChannel:                balanced.SideChannel,
		ServingModelWithProviderID: balanced.ServingModelWithProviderID,
		RealModelName:              balanced.RealModelName,
	}, res.Body)

	writeHeader(c, before.Stream, res.Header)
	// 响应头已提交后任何失败都不能再走 httpresp 收口（JSON 会污染已开始的流），
	// 收尾口径统一在 writeResponseBody：流式写协议内 error 事件，非流式静默截断。
	// 「头后无数据」类失败已在 service 提交门（commit-before-write）被拦下换下家，
	// 走到这里的中断都是提交后的残余窗口（上游断流/写客户端失败）。
	if copyErr := writeResponseBody(ctx, c, style, before.Stream, body); copyErr != nil {
		// 【临时取样 · Stage 44 · 取样结束即删】「日志管道提前收尾」（io.ErrClosedPipe）这一格
		// 在本仓**没有任何既有留痕**：handler 依 framework-conventions §5 边界③ 刻意不自记，
		// 而日志侧此刻已经收工、该错误只被回灌给无人读的管道——故无法从库里反推它到底发生过
		// 没有（全库 29,226 行日志里含 closed pipe 的 0 行）。带 log_id 是为了事后回 chat_logs
		// 对照该行的终态（success = 日志侧先收工），据此定这一格该记供应商还是记调用方
		// （见 .local/find-issues/2026-10-04-tee收尾只改了统计面清钉与留痕级别仍走Stage24的断连判据.md）。
		slog.Warn("post-commit write failure (Stage 44 sampling)",
			"log_id", balanced.LogID,
			"request_id", reqMeta.RequestID,
			"style", style,
			"stream", before.Stream,
			"closed_pipe", errors.Is(copyErr, io.ErrClosedPipe),
			"ctx_err", ctx.Err(),
			"error", copyErr,
		)
		finishLog(copyErr)
		return
	}

	// 关闭写入端，让读取端知道数据已发送完
	finishLog(nil)
}

// attachLogRecord 登记日志后处理（bgtask）并按登记结果分流响应体（Stage 35 丢弃窗口）。
//
//   - 登记成功：响应体 tee 进日志管道，finishLog(err) 收尾管道（nil=正文写完的 EOF，
//     err=断流，经 CloseWithError 传给日志侧走失败留痕）。
//   - 登记被丢弃（bgtask 已置 shutdown，只在关机收尾段可达）：当场放手——关掉无人
//     消费的管道（不关则 tee 写端挂死在无读端的 io.Pipe 上，请求拖到进程退出）、
//     响应体直通客户端让请求马上收尾放行，并给该行日志留「未能写完」失败痕迹。
//     正文随任务丢弃是丢弃窗口的共识代价，行停在建行值 processing 才是失真。
//
// finishLog 必须在响应写完后恰好调用一次；丢弃路径的 finishLog 是空操作（管道已关），
// 调用方收尾姿势不分支。
func attachLogRecord(ctx context.Context, in service.RecordLogInput, resBody io.Reader) (io.Reader, func(error)) {
	pr, pw := io.Pipe()
	in.Reader = pr
	registered := bgtask.Go(func(taskCtx context.Context) {
		service.RecordLog(taskCtx, in)
	})
	if registered {
		// 打标在 tee **之外**：tee 往日志管道的写失败（如日志侧提前收尾的坏管道）不是
		// 上游故障，被混进标记会把「日志管道收尾」误判成「上游腰斩」而误清粘性。
		return io.TeeReader(service.MarkUpstreamReader(resBody), pw), func(err error) {
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			pw.Close()
		}
	}

	pr.Close()
	pw.Close()
	service.MarkLogUnfinished(ctx, in.LogID, in.ReqStart)
	return resBody, func(error) {}
}

// upstreamResponseHeadersAllowed 是回给客户端的上游响应头允许清单（单源），按**上游侧**
// 头名判定。网关只承诺两类：① Content-Type——协议形状，客户端靠它判 JSON / SSE；
// ② Retry-After——「多久后再来」。限流族头名开放，按前缀放行（见
// upstreamRateLimitHeaderPrefixes）；上游的报障编号（`X-Request-Id` 与 Anthropic 系的
// `request-id`）由 writeHeader 择一改名单独放行、**不查本表**——改名后的输出名
// （consts.HeaderUpstreamRequestID）只由那个分支产生，上游自称同名头一律丢弃：否则客户端
// 分不清「网关改名的供应商号」与「上游自写值」，上游同时发两个头时还会看到两个值（Stage 3
// 治的就是这种双语义）。
//
// 单消费方（全仓只有 writeHeader 把上游头写进客户端响应），故留在本包；一旦出现第二个
// 消费方，按请求方向那份名单的同款理由（common.PassthroughAllowedHeader 注释：两份
// 名单必然漂移）下沉 common/，不要复制一份。
//
// 为什么必须收口而不是裸拷上游头：上游回的每个头都会进调用方看到的响应，供应商
// 内部的节点名、内部追踪号、调试头一并外流；供应商换一家或改实现，它们就跟着变，
// 而调用方可能已经依赖上了。Content-Length / Content-Encoding 同样在剔除之列，但理由
// 是分开的两条：网关自身从不请求压缩（出站 Accept-Encoding 在 chatcore.BuildHeaders
// 统一剥除、由传输层透明解压），故 Content-Encoding 本就不会出现；而 Content-Length
// 是「上游即便给了也不能照抄」——正文经协议转换改写后长度会变，照抄会截断响应（identity
// 响应上游确实会带此头，漏掉它恰恰是踩坑处），长度交给 net/http 自己算。
var upstreamResponseHeadersAllowed = map[string]bool{
	"Content-Type": true,
	"Retry-After":  true,
}

// upstreamRateLimitHeaderPrefixes 限流族前缀（canonical 形态）：OpenAI 风格的
// x-ratelimit-* 与 anthropic-ratelimit-*。这类头名是开放的（remaining / reset /
// limit-requests…），逐个列举必然漏，故按前缀放行。
var upstreamRateLimitHeaderPrefixes = []string{"X-Ratelimit-", "Anthropic-Ratelimit-"}

// upstreamHeaderAllowed 判断一个上游响应头（canonical 名）能否回给客户端。
func upstreamHeaderAllowed(canonical string) bool {
	if upstreamResponseHeadersAllowed[canonical] {
		return true
	}
	for _, prefix := range upstreamRateLimitHeaderPrefixes {
		if strings.HasPrefix(canonical, prefix) {
			return true
		}
	}
	return false
}

func writeHeader(c *gin.Context, stream bool, header http.Header) {
	// 响应头 X-Request-Id 唯一归属网关（Stage 3：报障编号不允许双语义）。上游常自带报障编号
	// （Azure/OpenAI 系 `X-Request-Id`，Anthropic 系 `request-id`），直接 Add 会让客户端看到
	// [网关号, 供应商号] 两值、取哪个全凭 SDK 实现——故上游值一律改名回传，向供应商报障时仍
	// 拿得到原号。
	//
	// 先择一、再由本函数单独用 Set 写出（而非在循环里 Add）：两种上游拼法若同时出现，Add 会让
	// 同一个输出名带上两个值——那正是本条要治的双语义，只换了个头名。X-Request-Id 优先。
	vendorRequestID := header.Get(consts.HeaderRequestID)
	if vendorRequestID == "" {
		vendorRequestID = header.Get(consts.HeaderVendorRequestID)
	}

	for k, values := range header {
		canonical := http.CanonicalHeaderKey(k)
		// 两种上游拼法都已在上面择一，不再进循环；原名永不作为上游值发出去，输出名
		// （X-Upstream-Request-Id）也不接受上游自称——它只由本函数产生。
		if canonical == consts.HeaderRequestID || canonical == consts.HeaderVendorRequestID {
			continue
		}
		if !upstreamHeaderAllowed(canonical) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(canonical, value)
		}
	}

	if vendorRequestID != "" {
		c.Writer.Header().Set(consts.HeaderUpstreamRequestID, vendorRequestID)
	}

	if stream {
		// 媒体类型字面量与读侧判据同源（common.SSEContentType），「声明流式」与「判定流式」不会再分叉。
		c.Header("Content-Type", common.SSEContentType)
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
	}
	c.Writer.Flush()
}

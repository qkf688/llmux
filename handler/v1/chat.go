package v1

import (
	"context"
	"errors"
	"io"
	"net/http"
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
		ReqStart:     startReq,
		Processer:    postProcessor,
		LogID:        balanced.LogID,
		Before:       *before,
		IOLog:        providersWithMeta.IOLog,
		ProviderName: balanced.ProviderName,
		SideChannel:  balanced.SideChannel,
	}, res.Body)

	writeHeader(c, ctx, before.Stream, res.Header)
	// 响应头已提交后任何失败都不能再走 httpresp 收口（JSON 会污染已开始的流），
	// 收尾口径统一在 writeResponseBody：流式写协议内 error 事件，非流式静默截断。
	// 「头后无数据」类失败已在 service 提交门（commit-before-write）被拦下换下家，
	// 走到这里的中断都是提交后的残余窗口（上游断流/写客户端失败）。
	if copyErr := writeResponseBody(c, style, before.Stream, body); copyErr != nil {
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
//     正文随任务丢弃是丢弃窗口的共识代价，行停在建行占位 success 才是失真。
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
		return io.TeeReader(resBody, pw), func(err error) {
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

func writeHeader(c *gin.Context, ctx context.Context, stream bool, header http.Header) {
	// 检查是否需要移除不必要的响应头
	stripHeaders := service.GetStripResponseHeaders(ctx)

	// 需要保留的核心响应头
	essentialHeaders := map[string]bool{
		"Content-Type":      true,
		"X-Request-Id":      true,
		"X-Ratelimit-Limit": true,
	}

	for k, values := range header {
		// 如果开启了移除响应头选项，只保留必要的头
		if stripHeaders && !essentialHeaders[k] {
			continue
		}
		// 响应头 X-Request-Id 唯一归属网关（Stage 3：报障编号不允许双语义）。
		// 上游常自带同名头（Azure/OpenAI），Add 会让客户端看到 [网关号, 供应商号]
		// 两值、取哪个全凭 SDK 实现——上游值改名回传，向供应商报障时仍拿得到原号。
		out := k
		if http.CanonicalHeaderKey(k) == consts.HeaderRequestID {
			out = "X-Upstream-Request-Id"
		}
		for _, value := range values {
			c.Writer.Header().Add(out, value)
		}
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

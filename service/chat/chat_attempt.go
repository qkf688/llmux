package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/adjustment"
	"github.com/qkf688/llmux/service/chatcore"
	"github.com/qkf688/llmux/service/chatstats"
	"github.com/qkf688/llmux/service/transform"
)

func executeSingleProviderAttempt(input singleProviderAttemptInput, retryLog chan<- models.ChatLog) singleProviderAttemptResult {
	logEntry := models.ChatLog{
		Name:          input.Before.Model,
		RealModelName: input.RealModelName,
		ProviderModel: input.ModelWithProvider.ProviderModel,
		ProviderName:  input.Provider.Name,
		Status:        "success",
		Style:         input.Style,
		UserAgent:     input.ReqMeta.UserAgent,
		RemoteIP:      input.ReqMeta.RemoteIP,
		ChatIO:        input.IOLog,
		Retry:         input.Retry,
		// 仅作占位快照：此刻上游请求尚未发出，取到的只是准备阶段耗时。
		// 终值由各出口回填（成功在 RecordLog，失败在各 errorUpdate / 写 retryLog 前），
		// 否则所有日志的「代理耗时」恒为近零。
		// ⚠️ 强制约定：**任何新的终态出口必须先回填 ProxyTime 再落库**，否则该出口
		// 的日志耗时重新失真成近零——本字段的「唯一赋值点」必须保持在各出口之外无效。
		ProxyTime: time.Since(input.Start),

		// 调度明细（S3-3）：建行时从 Selection 填充一次即贯穿——失败出口全部走
		// struct Updates（零值跳过），不会用空串覆盖这些字段。Select 失败的候选
		// 淘汰路径不建行（无日志），故走到这里的行必有命中信息。
		EndpointProtocol: input.Selection.Endpoint.Protocol,
		EndpointURL:      input.Selection.UpstreamURL,
		KeyGroupName:     input.Selection.Group.Name,
		CredentialNote:   credentialLabel(input.Selection.Credential),
	}

	withHeader := false
	if input.ModelWithProvider.WithHeader != nil {
		withHeader = *input.ModelWithProvider.WithHeader
	}
	header := chatcore.BuildHeaders(input.ReqMeta.Header, withHeader, input.ModelWithProvider.CustomerHeaders, input.Before.Stream)

	reqCtx := withOptionalRequestTrace(input.Ctx)

	// 构建思考档位钳制配置：supportsThinking=false 时 ThinkingClamp 为 nil
	// （stripThinkingFields 已整体剥离 thinking，钳制无意义）。
	var thinkingClamp *transform.ThinkingClampConfig
	supportsThinking := input.ModelWithProvider.SupportsThinkingResolved(input.Model)
	if supportsThinking {
		thinkingClamp = buildThinkingClampConfig(input.Ctx, input.Model, &input.ModelWithProvider)
	}

	requestBody, skipProvider, bodyErr := buildRequestBodyForProvider(input.Ctx, ProviderRequestCaps{
		Style:                      input.Style,
		EndpointProtocol:           consts.Protocol(input.Selection.Endpoint.Protocol),
		Raw:                        input.Before.raw,
		MaxTokensLimit:             input.ModelWithProvider.MaxTokens,
		SupportsThinking:           supportsThinking,
		ThinkingClamp:              thinkingClamp,
		AllowBudgetExceedMaxTokens: providerAllowsBudgetExceedMaxTokens(input.ChatModel),
	})
	if skipProvider {
		return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
	}
	if bodyErr != nil {
		var statusCoder interface{ StatusCode() int }
		if errors.As(bodyErr, &statusCoder) {
			return singleProviderAttemptResult{FatalErr: bodyErr}
		}
		logEntry.ProxyTime = time.Since(input.Start) // 写通道前回填，终值随 Create 落库
		retryLog <- logEntry.WithError(fmt.Errorf("transform request error: %v", bodyErr))
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		return singleProviderAttemptResult{RemoveWeight: true}
	}

	logRawOptions := getLogRawRequestResponse(input.Ctx)
	logSnapshot := captureRequestLogSnapshot(logRawOptions, input.ReqMeta.Header, requestBody, input.Before.raw)

	req, err := input.ChatModel.BuildReq(reqCtx, header, input.ModelWithProvider.ProviderModel, requestBody)
	if err != nil {
		logEntry.ProxyTime = time.Since(input.Start) // 写通道前回填，终值随 Create 落库
		retryLog <- logEntry.WithError(err)
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		return singleProviderAttemptResult{RemoveWeight: true}
	}

	logID, err := SaveChatLog(input.Ctx, logEntry)
	if err != nil {
		slog.Error("failed to create log before request", "error", err)
		return singleProviderAttemptResult{FatalErr: err}
	}

	res, err := input.Client.Do(req)
	if err != nil {
		errorUpdate := models.ChatLog{
			Status: "error",
			Error:  err.Error(),
			// 回填端到端耗时：上游请求已发出，建行快照不含任何上游耗时。
			ProxyTime: time.Since(input.Start),
		}
		if logRawOptions.RequestHeaders {
			errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
		}
		if logRawOptions.RequestBody {
			errorUpdate.RequestBody = logSnapshot.RequestBodyStr
		}
		if logRawOptions.RawRequestBody {
			errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
		}
		updateChatLogByID(input.Ctx, logID, errorUpdate, "failed to update log status")
		// 组织级 adjustment 不再在此累计：网络/超时是凭据级失败（#13），由 retry loop
		// 组耗尽时统一补 ConsecutiveFailures/衰减（设计定案第 5 节「层内耗尽才累计
		// 组织级」）。chatstats 统计保留现状（凭据级失败也是该供应商请求失败）。
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		// 网络/超时属凭据级失败：写冷却，组内换 key 可能救回（#13）。
		// 组织级淘汰标记仍返回（现状语义不变，组耗尽才消费）。
		applyCredentialCooldown(input.Ctx, input.Selection.Credential, "network")
		return singleProviderAttemptResult{CredentialFailure: true, RemoveWeight: true, RemovePriority: true}
	}

	if res.StatusCode != http.StatusOK {
		return handleNonOKProviderResponse(input.Ctx, res, logID, logRawOptions, logSnapshot, input.ModelWithProvider, input.Provider, input.Start, input.Selection.Credential)
	}

	rawResponseBodyStr := captureRawResponseBody(logRawOptions, res)

	// 流式响应的原始 body 不能在入口读取（会阻塞流），改用侧信道在转换 goroutine 内旁路记录。
	// 侧信道同时承载**上游原始 usage**：转换后的下游流受目标协议表达能力限制，
	// 从它反解 usage 必然有损，故由转换层在解析上游时旁路交出真值（见 models.TransformSideChannel）。
	// 仅在需要转换时创建——直通路径没有转换层可供旁路，落库回退 processer 解析（该路径本就完整）。
	captureRawBody := logRawOptions.RawResponseBody && rawResponseBodyStr == ""

	// 响应侧的直通判定必须与请求侧（buildRequestBodyForProvider）用同一依据：**协议形状**。
	// 取数点与请求侧一致 = 选中端点协议（S3-2 起替代 Provider.Type）——按 provider
	// type 字符串比较会让「openai 客户端打一家 OpenAI 兼容的新上游」请求直通、
	// 响应却白跑一趟转换。任一侧形状解析不出来时按「形状不同」走转换路径，由转换层报错
	// （请求侧已先解析过并在失败时返回错误，走到这里两侧本应都解析成功）。
	clientFormat, clientFormatOK := consts.WireFormatOfStyle(consts.Style(input.Style))
	upstreamFormat, upstreamFormatOK := consts.WireFormatOfProtocol(consts.Protocol(input.Selection.Endpoint.Protocol))

	var sideChannel *models.TransformSideChannel
	if !clientFormatOK || !upstreamFormatOK || clientFormat != upstreamFormat {
		sideChannel = models.NewTransformSideChannel(captureRawBody)
		tm := transform.NewTransformerManager(clientFormat, upstreamFormat)
		convertedRes, err := tm.ProcessResponse(res, sideChannel)
		if err != nil {
			errorUpdate := models.ChatLog{
				Status: "error",
				Error:  fmt.Sprintf("transform response error: %v", err),
				// 同 Client.Do 失败分支：错误日志回填真实端到端耗时。
				ProxyTime: time.Since(input.Start),
			}
			if logRawOptions.ResponseHeaders {
				responseHeadersJSON, marshalErr := json.Marshal(res.Header)
				if marshalErr != nil {
					slog.Error("failed to marshal response headers", "error", marshalErr)
					responseHeadersJSON = []byte("{}")
				}
				errorUpdate.ResponseHeaders = string(responseHeadersJSON)
			}
			if logRawOptions.RequestHeaders {
				errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
			}
			if logRawOptions.RequestBody {
				errorUpdate.RequestBody = logSnapshot.RequestBodyStr
			}
			if logRawOptions.RawRequestBody {
				errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
			}
			if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
				errorUpdate.RawResponseBody = rawResponseBodyStr
			}
			updateChatLogByID(input.Ctx, logID, errorUpdate, "failed to update log status")
			applyProviderFailureAdjustments(input.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
			if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
				slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
			}
			res.Body.Close()
			return singleProviderAttemptResult{RemoveWeight: true}
		}
		res = convertedRes
	} else {
		slog.Debug("passthrough response", "client_format", clientFormat, "upstream_format", upstreamFormat, "endpoint_protocol", input.Selection.Endpoint.Protocol)
	}

	updateRequestAndResponseLog(input.Ctx, logID, logRawOptions, logSnapshot, res.Header, rawResponseBodyStr)

	adjustment.ResetConsecutiveFailures(input.Ctx, input.ModelWithProvider.ID)
	adjustment.ApplySuccessAdjustments(input.Ctx, input.ModelWithProvider.ID)
	return singleProviderAttemptResult{
		Response:    res,
		LogID:       logID,
		Success:     true,
		SideChannel: sideChannel,
	}
}

// credentialLabel 返回凭据的日志展示标识：Note 非空用 Note（用户可读），
// 否则取 KeyHash 前 8 位加 # 前缀——组内多 key 排查（故障转移日志）需要区分
// 命中的是哪条凭据，KeyHash 是不解密即可获得的稳定脱敏标识。# 前缀把「哈希
// 派生标识」与「用户手填 Note」在展示上区分开。KeyHash 也缺失（构造形态）时
// 用显式占位符，避免落一条无意义的光杆 "#"。
func credentialLabel(c models.Credential) string {
	if c.Note != "" {
		return c.Note
	}
	if len(c.KeyHash) >= 8 {
		return "#" + c.KeyHash[:8]
	}
	if c.KeyHash != "" {
		return "#" + c.KeyHash
	}
	return "(unknown)"
}

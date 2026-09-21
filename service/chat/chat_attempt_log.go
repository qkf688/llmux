package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
)

func captureRequestLogSnapshot(options models.RawLogOptions, requestHeader http.Header, requestBody []byte, rawRequestBody []byte, fieldLimit int) requestLogSnapshot {
	snapshot := requestLogSnapshot{}
	if !options.RequestHeaders && !options.RequestBody && !options.RawRequestBody {
		return snapshot
	}

	if options.RequestHeaders {
		// Stage 16：marshal 前先过敏感头剥除（判据单源 common.MaskSensitiveLogHeaders，
		// deny-list 值掩码、键保留）。快照是 ReqMeta.Header 的下游——该引用与出站透传
		// 共享，剥除函数克隆语义保证不污染真实出站头；全仓 RequestHeaders 只有本
		// marshal 点一个写入源，此处接线即覆盖全部落库出口。
		requestHeadersJSON, err := json.Marshal(common.MaskSensitiveLogHeaders(requestHeader))
		if err != nil {
			slog.Error("failed to marshal request headers", "error", err)
			requestHeadersJSON = []byte("{}")
		}
		snapshot.RequestHeadersJSON = requestHeadersJSON
	}

	// Stage 2-D：请求侧采集同样受单字段上限约束（base64 大图的入站请求体原样进库
	// 与响应侧撑库是同一个病），截断走 common.TruncateUTF8 单源。
	if options.RequestBody {
		snapshot.RequestBodyStr = common.TruncateUTF8(string(requestBody), fieldLimit)
	}

	if options.RawRequestBody {
		snapshot.RawRequestBodyStr = common.TruncateUTF8(string(rawRequestBody), fieldLimit)
	}

	return snapshot
}

// captureRawResponseBody 采集原始响应体。**只做读头截、不吞尾**：本函数拿到的
// res.Body 是上游体积闸回填的内存字节（非流式必经闸、SSE 已在此跳过），LimitReader
// 只取采集上限内的头部，MultiReader 把「头部副本 + 剩余流」原样交还下游——
// 采集是旁路，不能成为转换/直通读取的截断点（Stage 2-D）。
func captureRawResponseBody(ctx context.Context, options models.RawLogOptions, res *http.Response) string {
	if !options.RawResponseBody {
		return ""
	}

	// For SSE streams, avoid reading the entire body here (it would block streaming and buffer unbounded data).
	// 判据单源 common.IsEventStreamContentType（与网关分流/探针同一条件，见 #3 收敛）。
	if common.IsEventStreamContentType(res.Header.Get("Content-Type")) {
		return ""
	}

	limit := getRawLogFieldMaxBytes(ctx)
	// 多探 1 字节：正文恰好等于上限时也必须能区分「完整」与「尾部被切」——
	// 只有读到 limit+1 才知道还有尾（与入站/上游闸同一探测法）。
	head, err := io.ReadAll(io.LimitReader(res.Body, int64(limit)+1))
	if err != nil {
		slog.Error("failed to read raw response body", "error", err)
		return ""
	}

	res.Body = io.NopCloser(io.MultiReader(bytes.NewReader(head), res.Body))
	return common.TruncateUTF8(string(head), limit)
}

func updateRequestAndResponseLog(
	ctx context.Context,
	logID uint,
	logRawOptions models.RawLogOptions,
	logSnapshot requestLogSnapshot,
	responseHeader http.Header,
	rawResponseBodyStr string,
) {
	if !logRawOptions.RequestHeaders && !logRawOptions.RequestBody && !logRawOptions.RawRequestBody && !logRawOptions.ResponseHeaders && !logRawOptions.RawResponseBody {
		return
	}

	updateData := models.ChatLog{}

	if logRawOptions.ResponseHeaders {
		responseHeadersJSON, err := json.Marshal(responseHeader)
		if err != nil {
			slog.Error("failed to marshal response headers", "error", err)
			responseHeadersJSON = []byte("{}")
		}
		updateData.ResponseHeaders = string(responseHeadersJSON)
	}

	if logRawOptions.RequestHeaders {
		updateData.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
	}
	if logRawOptions.RequestBody {
		updateData.RequestBody = logSnapshot.RequestBodyStr
	}
	if logRawOptions.RawRequestBody {
		updateData.RawRequestBody = logSnapshot.RawRequestBodyStr
	}
	if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
		updateData.RawResponseBody = rawResponseBodyStr
	}

	updateChatLogByID(ctx, logID, updateData, "failed to update log with request/response headers")
}

// updateChatLogByID 是 chat_logs 失败出口/回写落库的单源收口（Stage 17）：
// 写库 ctx 经 context.WithoutCancel + chatLogWriteTimeout 派生——超时/断连
// 场景请求 ctx 已死，陪葬会让「最需要留痕的失败」静默丢失（行停在建行占位的
// success）。全部失败出口与成功路径 raw 回写都经此处，未来新增出口自动被
// 覆盖（OCP）。写库失败仅 slog 留痕，不改调用方返回语义（请求本已注定失败，
// 日志是副作用）。update 保持 struct Updates 姿势——零值跳过是调度明细/
// 归因快照穿透不被覆盖的设计依赖，禁止改 map。
func updateChatLogByID(ctx context.Context, logID uint, update models.ChatLog, errorMsg string) {
	if logID == 0 {
		// disable_all_logs 时建行返回 0：不存在的行不做空转写库。
		return
	}
	writeCtx, cancel := chatLogDetachedCtx(ctx)
	defer cancel()
	if _, err := repos().ChatLog.UpdateByID(writeCtx, logID, update); err != nil {
		slog.Error(errorMsg, "log_id", logID, "error", err)
	}
}

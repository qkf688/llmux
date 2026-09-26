package importexport

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// ImportConfig 导入系统配置（整段同一事务；半残写入语义原样保留）。
func ImportConfig(c *gin.Context) {
	// 入站硬上限复用 consts.MaxInboundBodyBytes（Stage 2：管理端也堵裸 ReadAll，
	// 免得质量门约定里留一个注释豁免例外）。超限走 BadRequest 信封而非真实 413：
	// 管理端信封惯例是「HTTP 200 + 信封 code」，与同域导出端点及 /v1 的 413 各自
	// 对齐既有形状（前端两条通道都会解析，改动原因与前端无关）。
	// 4xx 留痕 ≤Warn（framework-conventions §5 级别约定）：客户端错误不是服务器
	// 故障，按 ERROR 计数的告警会被坏客户端刷爆。
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, consts.MaxInboundBodyBytes+1))
	if err != nil {
		slog.Warn("读取请求体失败", "error", err)
		httpresp.BadRequest(c, "读取请求体失败")
		return
	}
	defer c.Request.Body.Close()
	if int64(len(body)) > consts.MaxInboundBodyBytes {
		slog.Warn("导入请求体超限", "bytes", len(body), "limit", consts.MaxInboundBodyBytes)
		httpresp.BadRequest(c, "请求体超过上限 20 MiB")
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		slog.Warn("解析配置数据失败", "error", err)
		httpresp.BadRequest(c, "解析配置数据失败")
		return
	}

	var outcome ImportOutcome
	// 事务收口走 GORM Transaction 闭包：返回 error 回滚、panic 回滚后**继续上抛**
	// 给 middleware.Recovery（500 + [req=] + 栈进日志）。此前手写 defer recover
	// 只回滚不留痕——panic 被吞成空响应，且回滚后 handler 继续走完、对外像成功。
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		outcome, err = ImportConfigData(tx, config)
		return err
	}); err != nil {
		var ie *ImportError
		if errors.As(err, &ie) {
			httpresp.InternalErrorMessage(c, ie.Message, ie.Err)
			return
		}
		httpresp.InternalErrorMessage(c, "导入配置失败", err)
		return
	}

	slog.Info("配置导入成功")
	// 回颗粒度计数：前端按类型汇总展示「已导入: 提供商 N 条」。
	// templates 不在导入注册表内，恒零值（前端据零值不渲染该项）。
	httpresp.Success(c, outcome)
}

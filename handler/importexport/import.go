package importexport

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
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
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, consts.MaxInboundBodyBytes+1))
	if err != nil {
		slog.Error("读取请求体失败", "error", err)
		httpresp.BadRequest(c, "读取请求体失败")
		return
	}
	defer c.Request.Body.Close()
	if int64(len(body)) > consts.MaxInboundBodyBytes {
		slog.Error("导入请求体超限", "bytes", len(body), "limit", consts.MaxInboundBodyBytes)
		httpresp.BadRequest(c, "请求体超过上限 20 MiB")
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		slog.Error("解析配置数据失败", "error", err)
		httpresp.BadRequest(c, "解析配置数据失败")
		return
	}

	tx := models.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := ImportConfigData(tx, config); err != nil {
		tx.Rollback()
		var ie *ImportError
		if errors.As(err, &ie) {
			httpresp.InternalErrorMessage(c, ie.Message, ie.Err)
			return
		}
		slog.Error("导入配置失败", "error", err)
		httpresp.InternalErrorMessage(c, "导入配置失败", err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("提交事务失败", "error", err)
		httpresp.InternalErrorMessage(c, "提交事务失败", err)
		return
	}

	slog.Info("配置导入成功")
	httpresp.Success(c, gin.H{"message": "配置导入成功"})
}

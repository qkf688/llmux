package importexport

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// ImportConfig 导入系统配置（整段同一事务；半残写入语义原样保留）。
func ImportConfig(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("读取请求体失败", "error", err)
		httpresp.BadRequest(c, "读取请求体失败")
		return
	}
	defer c.Request.Body.Close()

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
			slog.Error(ie.Message, "error", ie.Err)
			httpresp.InternalServerError(c, ie.Message)
			return
		}
		slog.Error("导入配置失败", "error", err)
		httpresp.InternalServerError(c, "导入配置失败")
		return
	}

	if err := tx.Commit().Error; err != nil {
		slog.Error("提交事务失败", "error", err)
		httpresp.InternalServerError(c, "提交事务失败")
		return
	}

	slog.Info("配置导入成功")
	httpresp.Success(c, gin.H{"message": "配置导入成功"})
}

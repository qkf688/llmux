package settings

import (
	"github.com/atopos31/llmio/httpresp"
	"github.com/gin-gonic/gin"
)

// GetSystemConfig 获取系统配置
func GetSystemConfig(c *gin.Context) {
	config := map[string]interface{}{
		"enable_smart_routing":  true,
		"success_rate_weight":   0.7,
		"response_time_weight":  0.3,
		"decay_threshold_hours": 24,
		"min_weight":            1,
	}

	httpresp.Success(c, config)
}

// UpdateSystemConfig 更新系统配置
func UpdateSystemConfig(c *gin.Context) {
	var req SystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	config := map[string]interface{}{
		"enable_smart_routing":  req.EnableSmartRouting,
		"success_rate_weight":   req.SuccessRateWeight,
		"response_time_weight":  req.ResponseTimeWeight,
		"decay_threshold_hours": req.DecayThresholdHours,
		"min_weight":            req.MinWeight,
	}

	httpresp.Success(c, config)
}

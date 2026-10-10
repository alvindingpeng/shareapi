package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// GetSmartRoutingSetting handles GET /api/option/smart_routing.
func GetSmartRoutingSetting(c *gin.Context) {
	s := operation_setting.GetSmartRoutingSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":  s.Enabled,
			"strategy": s.Strategy,
		},
	})
}

// UpdateSmartRoutingSetting handles PATCH /api/option/smart_routing.
func UpdateSmartRoutingSetting(c *gin.Context) {
	var req struct {
		Enabled  *bool   `json:"enabled"`
		Strategy *string `json:"strategy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	s := operation_setting.GetSmartRoutingSetting()
	enabled := s.Enabled
	strategy := s.Strategy
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Strategy != nil {
		strategy = *req.Strategy
	}
	operation_setting.UpdateSmartRoutingSetting(enabled, strategy)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":  enabled,
			"strategy": operation_setting.GetSmartRoutingSetting().Strategy,
		},
	})
}

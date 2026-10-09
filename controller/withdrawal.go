package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Contributor ledger & withdrawal APIs (Phase 5 funding layer).

// GetLedgerBalance handles GET /api/contributor/ledger/balance.
func GetLedgerBalance(c *gin.Context) {
	contributorID := contributorID(c)
	matured, err := model.GetMaturedBalance(contributorID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pending, err := model.GetPendingBalance(contributorID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"matured": matured,
			"pending": pending,
			"total":   matured + pending,
		},
	})
}

// RequestWithdrawal handles POST /api/contributor/withdrawals.
func RequestWithdrawal(c *gin.Context) {
	var input struct {
		Amount int64  `json:"amount"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	w, err := model.RequestWithdrawal(contributorID(c), input.Amount, input.Note)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": w})
}

// ListWithdrawals handles GET /api/contributor/withdrawals (own requests).
func ListWithdrawals(c *gin.Context) {
	var list []model.Withdrawal
	if err := model.DB.Where("contributor_id = ?", contributorID(c)).
		Order("id DESC").Find(&list).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// AdminListWithdrawals handles GET /api/withdrawals (admin).
func AdminListWithdrawals(c *gin.Context) {
	var list []model.Withdrawal
	query := model.DB.Order("id DESC")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&list).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// AdminReviewWithdrawal handles POST /api/withdrawals/:id/review (admin).
func AdminReviewWithdrawal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Approve bool `json:"approve"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	adminID, _ := c.Get("id")
	reviewerID, _ := adminID.(int)
	if err := model.ReviewWithdrawal(id, reviewerID, input.Approve); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "reviewed"})
}

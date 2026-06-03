package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ScheduledTestHandler handles admin scheduled-test-plan management.
type ScheduledTestHandler struct {
	scheduledTestSvc *service.ScheduledTestService
	runnerSnapshots  func() []service.ScheduledTestRunnerSnapshot
}

// NewScheduledTestHandler creates a new ScheduledTestHandler.
func NewScheduledTestHandler(scheduledTestSvc *service.ScheduledTestService) *ScheduledTestHandler {
	return &ScheduledTestHandler{scheduledTestSvc: scheduledTestSvc}
}

// SetRunnerService injects the scheduled runner status provider.
func (h *ScheduledTestHandler) SetRunnerService(runner *service.ScheduledTestRunnerService) {
	if h == nil || runner == nil {
		return
	}
	h.runnerSnapshots = runner.Snapshots
}

type createScheduledTestPlanRequest struct {
	AccountID           int64  `json:"account_id" binding:"required"`
	TaskType            string `json:"task_type"`
	ModelID             string `json:"model_id"`
	CronExpression      string `json:"cron_expression" binding:"required"`
	Enabled             *bool  `json:"enabled"`
	MaxResults          int    `json:"max_results"`
	AutoRecover         *bool  `json:"auto_recover"`
	ProbeMode           string `json:"probe_mode"`
	ProbeRequestMode    string `json:"probe_request_mode"`
	ProbeCodexStability bool   `json:"probe_codex_stability"`
	ProbeLongContext    bool   `json:"probe_long_context"`
}

type updateScheduledTestPlanRequest struct {
	TaskType            string `json:"task_type"`
	ModelID             string `json:"model_id"`
	CronExpression      string `json:"cron_expression"`
	Enabled             *bool  `json:"enabled"`
	MaxResults          int    `json:"max_results"`
	AutoRecover         *bool  `json:"auto_recover"`
	ProbeMode           string `json:"probe_mode"`
	ProbeRequestMode    string `json:"probe_request_mode"`
	ProbeCodexStability *bool  `json:"probe_codex_stability"`
	ProbeLongContext    *bool  `json:"probe_long_context"`
}

// ListByAccount GET /admin/accounts/:id/scheduled-test-plans
func (h *ScheduledTestHandler) ListByAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	plans, err := h.scheduledTestSvc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// Create POST /admin/scheduled-test-plans
func (h *ScheduledTestHandler) Create(c *gin.Context) {
	var req createScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	plan := &service.ScheduledTestPlan{
		AccountID:           req.AccountID,
		TaskType:            req.TaskType,
		ModelID:             req.ModelID,
		CronExpression:      req.CronExpression,
		Enabled:             true,
		MaxResults:          req.MaxResults,
		ProbeMode:           req.ProbeMode,
		ProbeRequestMode:    req.ProbeRequestMode,
		ProbeCodexStability: req.ProbeCodexStability,
		ProbeLongContext:    req.ProbeLongContext,
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}
	if req.AutoRecover != nil {
		plan.AutoRecover = *req.AutoRecover
	}

	created, err := h.scheduledTestSvc.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

// Update PUT /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	existing, err := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req updateScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.ModelID != "" {
		existing.ModelID = req.ModelID
	}
	if req.TaskType != "" {
		existing.TaskType = req.TaskType
	}
	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.MaxResults > 0 {
		existing.MaxResults = req.MaxResults
	}
	if req.AutoRecover != nil {
		existing.AutoRecover = *req.AutoRecover
	}
	if req.ProbeMode != "" {
		existing.ProbeMode = req.ProbeMode
	}
	if req.ProbeRequestMode != "" {
		existing.ProbeRequestMode = req.ProbeRequestMode
	}
	if req.ProbeCodexStability != nil {
		existing.ProbeCodexStability = *req.ProbeCodexStability
	}
	if req.ProbeLongContext != nil {
		existing.ProbeLongContext = *req.ProbeLongContext
	}

	updated, err := h.scheduledTestSvc.UpdatePlan(c.Request.Context(), existing)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

// Delete DELETE /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	if err := h.scheduledTestSvc.DeletePlan(c.Request.Context(), planID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListResults GET /admin/scheduled-test-plans/:id/results
func (h *ScheduledTestHandler) ListResults(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	results, err := h.scheduledTestSvc.ListResults(c.Request.Context(), planID, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

// ListRunnerSnapshots GET /admin/scheduled-test-runner/snapshots
func (h *ScheduledTestHandler) ListRunnerSnapshots(c *gin.Context) {
	if h.runnerSnapshots == nil {
		c.JSON(http.StatusOK, []service.ScheduledTestRunnerSnapshot{})
		return
	}
	c.JSON(http.StatusOK, h.runnerSnapshots())
}

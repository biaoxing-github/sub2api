package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type APIKeyProbeService interface {
	Run(ctx context.Context, req service.APIKeyProbeRunRequest) (service.APIKeyProbeResult, error)
	List(ctx context.Context, filter service.APIKeyProbeHistoryFilter) ([]service.APIKeyProbeResult, error)
	Get(ctx context.Context, userID, apiKeyID, runID int64) (*service.APIKeyProbeResult, error)
}

type APIKeyProbeHandler struct {
	probeService APIKeyProbeService
}

func NewAPIKeyProbeHandler(probeService APIKeyProbeService) *APIKeyProbeHandler {
	return &APIKeyProbeHandler{probeService: probeService}
}

type CreateAPIKeyProbeRunRequest struct {
	Mode                  string `json:"mode"`
	Model                 string `json:"model"`
	IncludeCodexStability bool   `json:"include_codex_stability"`
	IncludeLongContext    bool   `json:"include_long_context"`
	CodexStability        bool   `json:"codex_stability"`
	LongContext           bool   `json:"long_context"`
}

func (h *APIKeyProbeHandler) CreateProbeRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}
	var req CreateAPIKeyProbeRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = service.APIKeyProbeProfileStandard
	}
	result, err := h.probeService.Run(c.Request.Context(), service.APIKeyProbeRunRequest{
		UserID:                subject.UserID,
		APIKeyID:              keyID,
		Profile:               mode,
		Model:                 req.Model,
		IncludeCodexStability: req.IncludeCodexStability || req.CodexStability,
		IncludeLongContext:    req.IncludeLongContext || req.LongContext,
		BaseURL:               requestBaseURL(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *APIKeyProbeHandler) Run(c *gin.Context) {
	h.CreateProbeRun(c)
}

func (h *APIKeyProbeHandler) ListProbeRuns(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	result, err := h.probeService.List(c.Request.Context(), service.APIKeyProbeHistoryFilter{
		UserID:   subject.UserID,
		APIKeyID: keyID,
		Limit:    limit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *APIKeyProbeHandler) GetProbeRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid probe run ID")
		return
	}
	result, err := h.probeService.Get(c.Request.Context(), subject.UserID, keyID, runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = strings.Split(proto, ",")[0]
	} else if c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	if forwardedHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.Split(forwardedHost, ",")[0]
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + strings.TrimSpace(host)
}

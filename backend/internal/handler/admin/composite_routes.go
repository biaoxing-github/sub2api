package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GroupHandler) compositeAdmin() service.CompositeRouteAdminService {
	return h.adminService.(service.CompositeRouteAdminService)
}

type compositeRouteRequest struct {
	PublicModel    string `json:"public_model" binding:"required"`
	MatchType      string `json:"match_type"`
	TargetPlatform string `json:"target_platform" binding:"required"`
	UpstreamModel  string `json:"upstream_model"`
	Endpoint       string `json:"endpoint"`
	Priority       int    `json:"priority"`
	Enabled        *bool  `json:"enabled"`
	Notes          string `json:"notes"`
}
type compositeRoutePreviewRequest struct {
	Model    string `json:"model" binding:"required"`
	Endpoint string `json:"endpoint"`
}

func compositeRouteID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}
func compositeRouteInput(r compositeRouteRequest, enabledDefault bool) service.CompositeRouteInput {
	enabled := enabledDefault
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return service.CompositeRouteInput{PublicModel: r.PublicModel, MatchType: r.MatchType, TargetPlatform: r.TargetPlatform, UpstreamModel: r.UpstreamModel, Endpoint: r.Endpoint, Priority: r.Priority, Enabled: enabled, Notes: r.Notes}
}
func (h *GroupHandler) ListCompositeRoutes(c *gin.Context) {
	id, ok := compositeRouteID(c, "id")
	if !ok {
		return
	}
	v, err := h.compositeAdmin().ListCompositeRoutes(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, v)
}
func (h *GroupHandler) CreateCompositeRoute(c *gin.Context) {
	id, ok := compositeRouteID(c, "id")
	if !ok {
		return
	}
	var r compositeRouteRequest
	if c.ShouldBindJSON(&r) != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	v, err := h.compositeAdmin().CreateCompositeRoute(c.Request.Context(), id, compositeRouteInput(r, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, v)
}
func (h *GroupHandler) UpdateCompositeRoute(c *gin.Context) {
	id, ok := compositeRouteID(c, "id")
	if !ok {
		return
	}
	rid, ok := compositeRouteID(c, "route_id")
	if !ok {
		return
	}
	var r compositeRouteRequest
	if c.ShouldBindJSON(&r) != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	v, err := h.compositeAdmin().UpdateCompositeRoute(c.Request.Context(), id, rid, compositeRouteInput(r, true))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, v)
}
func (h *GroupHandler) DeleteCompositeRoute(c *gin.Context) {
	id, ok := compositeRouteID(c, "id")
	if !ok {
		return
	}
	rid, ok := compositeRouteID(c, "route_id")
	if !ok {
		return
	}
	if err := h.compositeAdmin().DeleteCompositeRoute(c.Request.Context(), id, rid); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "composite route deleted"})
}
func (h *GroupHandler) PreviewCompositeRoute(c *gin.Context) {
	id, ok := compositeRouteID(c, "id")
	if !ok {
		return
	}
	var r compositeRoutePreviewRequest
	if c.ShouldBindJSON(&r) != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	v, err := h.compositeAdmin().PreviewCompositeRoute(c.Request.Context(), id, service.CompositeRoutePreviewRequest{Model: r.Model, Endpoint: r.Endpoint})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, v)
}

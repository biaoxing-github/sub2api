package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type compositeRoutingRepoStub struct {
	routes []service.CompositeModelRoute
}

func (s *compositeRoutingRepoStub) ListByGroup(context.Context, int64, bool) ([]service.CompositeModelRoute, error) {
	return append([]service.CompositeModelRoute(nil), s.routes...), nil
}
func (s *compositeRoutingRepoStub) Create(context.Context, *service.CompositeModelRoute) error {
	return nil
}
func (s *compositeRoutingRepoStub) Update(context.Context, *service.CompositeModelRoute) error {
	return nil
}
func (s *compositeRoutingRepoStub) Delete(context.Context, int64) error        { return nil }
func (s *compositeRoutingRepoStub) DeleteByGroup(context.Context, int64) error { return nil }

func TestResolveCompositeRequestRewritesModelAndContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-pro"}`))
	resolver := service.NewCompositeRouteResolver(&compositeRoutingRepoStub{routes: []service.CompositeModelRoute{{
		ID: 1, GroupID: 7, PublicModel: "public-pro", MatchType: service.CompositeRouteMatchExact,
		TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5.2-pro", Endpoint: service.CompositeRouteEndpointResponses, Enabled: true,
	}}})
	apiKey := &service.APIKey{Group: &service.Group{ID: 7, Platform: service.PlatformComposite}}

	body, model, err := resolveCompositeRequest(c, resolver, apiKey, "public-pro", service.CompositeRouteEndpointResponses, []byte(`{"model":"public-pro"}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-5.2-pro", model)
	require.Equal(t, "gpt-5.2-pro", gjson.GetBytes(body, "model").String())
	platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, service.PlatformOpenAI, platform)
	upstream, ok := service.ResolvedUpstreamModelFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, "gpt-5.2-pro", upstream)
}

func TestResolveCompositeRequestFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"unknown-alias"}`))
	resolver := service.NewCompositeRouteResolver(&compositeRoutingRepoStub{})
	apiKey := &service.APIKey{Group: &service.Group{ID: 7, Platform: service.PlatformComposite}}

	_, _, err := resolveCompositeRequest(c, resolver, apiKey, "unknown-alias", service.CompositeRouteEndpointMessages, []byte(`{"model":"unknown-alias"}`))
	require.Error(t, err)
}

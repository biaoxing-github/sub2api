package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type compositeRouteRepoStub struct {
	routes []CompositeModelRoute
}

func (s compositeRouteRepoStub) ListByGroup(_ context.Context, groupID int64, includeDisabled bool) ([]CompositeModelRoute, error) {
	routes := make([]CompositeModelRoute, 0, len(s.routes))
	for _, route := range s.routes {
		if route.GroupID == groupID && (includeDisabled || route.Enabled) {
			routes = append(routes, route)
		}
	}
	return routes, nil
}

func (compositeRouteRepoStub) Create(context.Context, *CompositeModelRoute) error { return nil }
func (compositeRouteRepoStub) Update(context.Context, *CompositeModelRoute) error { return nil }
func (compositeRouteRepoStub) Delete(context.Context, int64) error                { return nil }
func (compositeRouteRepoStub) DeleteByGroup(context.Context, int64) error         { return nil }

func TestCompositeRouteResolverExplicitExactRouteRewritesModel(t *testing.T) {
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{
		ID: 10, GroupID: 7, PublicModel: "openrouter/gpt-5", MatchType: CompositeRouteMatchExact,
		TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointAny,
		Priority: 100, Enabled: true,
	}}})

	decision, err := resolver.Resolve(context.Background(), 7, "openrouter/gpt-5", CompositeRouteEndpointChatCompletions)

	require.NoError(t, err)
	require.True(t, decision.Matched)
	require.Equal(t, CompositeRouteSourceExplicit, decision.Source)
	require.Equal(t, PlatformOpenAI, decision.TargetPlatform)
	require.Equal(t, "gpt-5", decision.UpstreamModel)
	require.NotNil(t, decision.Route)
	require.Equal(t, int64(10), decision.Route.ID)
}

func TestCompositeRouteResolverPrefersEndpointSpecificLongestPrefix(t *testing.T) {
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 1, GroupID: 7, PublicModel: "router/", MatchType: CompositeRouteMatchPrefix, TargetPlatform: PlatformAnthropic, Endpoint: CompositeRouteEndpointAny, Priority: 10, Enabled: true},
		{ID: 2, GroupID: 7, PublicModel: "router/gpt-", MatchType: CompositeRouteMatchPrefix, TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-family", Endpoint: CompositeRouteEndpointResponses, Priority: 100, Enabled: true},
	}})

	decision, err := resolver.Resolve(context.Background(), 7, "router/gpt-5", CompositeRouteEndpointResponses)

	require.NoError(t, err)
	require.True(t, decision.Matched)
	require.Equal(t, CompositeRouteSourceExplicit, decision.Source)
	require.Equal(t, PlatformOpenAI, decision.TargetPlatform)
	require.Equal(t, "gpt-family", decision.UpstreamModel)
	require.NotNil(t, decision.Route)
	require.Equal(t, int64(2), decision.Route.ID)
}

func TestCompositeRouteResolverIgnoresDisabledRoutesAndFallsBackToDetector(t *testing.T) {
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{
		ID: 1, GroupID: 7, PublicModel: "gpt-5", MatchType: CompositeRouteMatchExact,
		TargetPlatform: PlatformAnthropic, UpstreamModel: "claude-sonnet-4-6", Endpoint: CompositeRouteEndpointAny,
		Priority: 100, Enabled: false,
	}}})

	decision, err := resolver.Resolve(context.Background(), 7, "gpt-5", CompositeRouteEndpointAny)

	require.NoError(t, err)
	require.True(t, decision.Matched)
	require.Equal(t, CompositeRouteSourceDetector, decision.Source)
	require.Equal(t, PlatformOpenAI, decision.TargetPlatform)
	require.Equal(t, "gpt-5", decision.UpstreamModel)
	require.Nil(t, decision.Route)
}

func TestCompositeRouteResolverPrefersExactRouteBeforeHigherPriorityPrefix(t *testing.T) {
	routes := []CompositeModelRoute{
		{ID: 1, GroupID: 7, PublicModel: "router/", MatchType: CompositeRouteMatchPrefix, TargetPlatform: PlatformAnthropic, Endpoint: CompositeRouteEndpointAny, Priority: 1, Enabled: true},
		{ID: 2, GroupID: 7, PublicModel: "router/gpt-5", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointAny, Priority: 999, Enabled: true},
	}

	decision, err := NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes}).Resolve(context.Background(), 7, "router/gpt-5", CompositeRouteEndpointResponses)

	require.NoError(t, err)
	require.Equal(t, int64(2), decision.Route.ID)
}

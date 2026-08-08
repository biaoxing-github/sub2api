package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// CompositeRouteResolver 解析 Composite 分组的显式路由，并在未命中时使用内置平台检测器。
type CompositeRouteResolver struct {
	repo CompositeModelRouteRepository
}

// NewCompositeRouteResolver 创建 Composite 模型路由解析器。
func NewCompositeRouteResolver(repo CompositeModelRouteRepository) *CompositeRouteResolver {
	return &CompositeRouteResolver{repo: repo}
}

// Resolve 返回指定分组、模型和端点的实际平台及上游模型。
func (r *CompositeRouteResolver) Resolve(ctx context.Context, groupID int64, model, endpoint string) (CompositeRouteDecision, error) {
	model = strings.TrimSpace(model)
	endpoint = normalizeCompositeRouteEndpoint(endpoint)
	decision := CompositeRouteDecision{GroupID: groupID, PublicModel: model, Endpoint: endpoint}
	if model == "" {
		decision.Reason = "model is required"
		return decision, nil
	}

	if r != nil && r.repo != nil && groupID > 0 {
		routes, err := r.repo.ListByGroup(ctx, groupID, false)
		if err != nil {
			return decision, fmt.Errorf("list composite routes: %w", err)
		}
		if route, ok := matchCompositeRoute(routes, model, endpoint); ok {
			upstreamModel := strings.TrimSpace(route.UpstreamModel)
			if upstreamModel == "" {
				upstreamModel = model
			}
			return CompositeRouteDecision{Matched: true, Source: CompositeRouteSourceExplicit, GroupID: groupID, PublicModel: model, TargetPlatform: route.TargetPlatform, UpstreamModel: upstreamModel, Endpoint: endpoint, Route: &route}, nil
		}
	}

	if platform, ok := DetectModelPlatform(model); ok {
		return CompositeRouteDecision{Matched: true, Source: CompositeRouteSourceDetector, GroupID: groupID, PublicModel: model, TargetPlatform: platform, UpstreamModel: model, Endpoint: endpoint}, nil
	}
	decision.Reason = "no explicit route or built-in detector match"
	return decision, nil
}

// matchCompositeRoute 从可用路由中按匹配强度、端点、前缀长度、优先级和 ID 选出唯一结果。
func matchCompositeRoute(routes []CompositeModelRoute, model, endpoint string) (CompositeModelRoute, bool) {
	type candidate struct {
		route          CompositeModelRoute
		matchStrength  int
		endpointWeight int
		prefixLen      int
	}
	candidates := make([]candidate, 0, len(routes))
	for _, route := range routes {
		route.Endpoint = normalizeCompositeRouteEndpoint(route.Endpoint)
		if route.Endpoint != endpoint && route.Endpoint != CompositeRouteEndpointAny {
			continue
		}
		route.MatchType = normalizeCompositeRouteMatchType(route.MatchType)
		publicModel := strings.TrimSpace(route.PublicModel)
		if publicModel == "" {
			continue
		}
		matchStrength := 0
		switch route.MatchType {
		case CompositeRouteMatchExact:
			if publicModel != model {
				continue
			}
			matchStrength = 2
		case CompositeRouteMatchPrefix:
			if !strings.HasPrefix(model, publicModel) {
				continue
			}
			matchStrength = 1
		}
		endpointWeight := 0
		if route.Endpoint == endpoint {
			endpointWeight = 1
		}
		candidates = append(candidates, candidate{route: route, matchStrength: matchStrength, endpointWeight: endpointWeight, prefixLen: len(publicModel)})
	}
	if len(candidates) == 0 {
		return CompositeModelRoute{}, false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.matchStrength != b.matchStrength {
			return a.matchStrength > b.matchStrength
		}
		if a.endpointWeight != b.endpointWeight {
			return a.endpointWeight > b.endpointWeight
		}
		if a.prefixLen != b.prefixLen {
			return a.prefixLen > b.prefixLen
		}
		if a.route.Priority != b.route.Priority {
			return a.route.Priority < b.route.Priority
		}
		return a.route.ID < b.route.ID
	})
	return candidates[0].route, true
}

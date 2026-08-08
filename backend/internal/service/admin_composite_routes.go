package service

import (
	"context"
	"fmt"
)

type CompositeRouteAdminService interface {
	ListCompositeRoutes(context.Context, int64) ([]CompositeModelRoute, error)
	CreateCompositeRoute(context.Context, int64, CompositeRouteInput) (*CompositeModelRoute, error)
	UpdateCompositeRoute(context.Context, int64, int64, CompositeRouteInput) (*CompositeModelRoute, error)
	DeleteCompositeRoute(context.Context, int64, int64) error
	PreviewCompositeRoute(context.Context, int64, CompositeRoutePreviewRequest) (*CompositeRouteDecision, error)
}

func compositeRouteFromInput(groupID int64, input CompositeRouteInput) (*CompositeModelRoute, error) {
	input = normalizeCompositeRouteInput(input)
	if input.PublicModel == "" {
		return nil, fmt.Errorf("public_model is required")
	}
	if !isConcreteRequestPlatform(input.TargetPlatform) {
		return nil, fmt.Errorf("target_platform must be a concrete provider")
	}
	if input.Priority == 0 {
		input.Priority = 100
	}
	return &CompositeModelRoute{GroupID: groupID, PublicModel: input.PublicModel, MatchType: input.MatchType, TargetPlatform: input.TargetPlatform, UpstreamModel: input.UpstreamModel, Endpoint: input.Endpoint, Priority: input.Priority, Enabled: input.Enabled, Notes: input.Notes}, nil
}

func (s *adminServiceImpl) requireCompositeGroup(ctx context.Context, groupID int64) error {
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return err
	}
	if group.Platform != PlatformComposite {
		return fmt.Errorf("group %d is not a composite group", groupID)
	}
	return nil
}

func (s *adminServiceImpl) ListCompositeRoutes(ctx context.Context, groupID int64) ([]CompositeModelRoute, error) {
	if err := s.requireCompositeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if s.compositeRouteRepo == nil {
		return nil, fmt.Errorf("composite route repository is not configured")
	}
	return s.compositeRouteRepo.ListByGroup(ctx, groupID, true)
}

func (s *adminServiceImpl) CreateCompositeRoute(ctx context.Context, groupID int64, input CompositeRouteInput) (*CompositeModelRoute, error) {
	if err := s.requireCompositeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if s.compositeRouteRepo == nil {
		return nil, fmt.Errorf("composite route repository is not configured")
	}
	route, err := compositeRouteFromInput(groupID, input)
	if err != nil {
		return nil, err
	}
	if err := s.compositeRouteRepo.Create(ctx, route); err != nil {
		return nil, err
	}
	return route, nil
}

func (s *adminServiceImpl) UpdateCompositeRoute(ctx context.Context, groupID, routeID int64, input CompositeRouteInput) (*CompositeModelRoute, error) {
	if err := s.requireCompositeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if s.compositeRouteRepo == nil {
		return nil, fmt.Errorf("composite route repository is not configured")
	}
	routes, err := s.compositeRouteRepo.ListByGroup(ctx, groupID, true)
	if err != nil {
		return nil, err
	}
	found := false
	for _, route := range routes {
		if route.ID == routeID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrCompositeRouteNotFound
	}
	route, err := compositeRouteFromInput(groupID, input)
	if err != nil {
		return nil, err
	}
	route.ID = routeID
	if err := s.compositeRouteRepo.Update(ctx, route); err != nil {
		return nil, err
	}
	return route, nil
}

func (s *adminServiceImpl) DeleteCompositeRoute(ctx context.Context, groupID, routeID int64) error {
	if err := s.requireCompositeGroup(ctx, groupID); err != nil {
		return err
	}
	if s.compositeRouteRepo == nil {
		return fmt.Errorf("composite route repository is not configured")
	}
	routes, err := s.compositeRouteRepo.ListByGroup(ctx, groupID, true)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.ID == routeID {
			return s.compositeRouteRepo.Delete(ctx, routeID)
		}
	}
	return ErrCompositeRouteNotFound
}

func (s *adminServiceImpl) PreviewCompositeRoute(ctx context.Context, groupID int64, input CompositeRoutePreviewRequest) (*CompositeRouteDecision, error) {
	if err := s.requireCompositeGroup(ctx, groupID); err != nil {
		return nil, err
	}
	resolver := s.compositeResolver
	if resolver == nil {
		resolver = NewCompositeRouteResolver(s.compositeRouteRepo)
	}
	decision, err := resolver.Resolve(ctx, groupID, input.Model, input.Endpoint)
	if err != nil {
		return nil, err
	}
	return &decision, nil
}

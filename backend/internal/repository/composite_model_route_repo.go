package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/compositemodelroute"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// compositeModelRouteRepository 使用 Ent 持久化 Composite 模型路由。
type compositeModelRouteRepository struct {
	client *dbent.Client
}

// NewCompositeModelRouteRepository 创建 Composite 模型路由仓储。
func NewCompositeModelRouteRepository(client *dbent.Client) service.CompositeModelRouteRepository {
	return &compositeModelRouteRepository{client: client}
}

// ListByGroup 按优先级和 ID 稳定返回指定分组的路由。
func (r *compositeModelRouteRepository) ListByGroup(ctx context.Context, groupID int64, includeDisabled bool) ([]service.CompositeModelRoute, error) {
	q := clientFromContext(ctx, r.client).CompositeModelRoute.Query().
		Where(compositemodelroute.GroupIDEQ(groupID)).
		Order(dbent.Asc(compositemodelroute.FieldPriority), dbent.Asc(compositemodelroute.FieldID))
	if !includeDisabled {
		q = q.Where(compositemodelroute.EnabledEQ(true))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.CompositeModelRoute, 0, len(rows))
	for _, row := range rows {
		out = append(out, *compositeModelRouteEntityToService(row))
	}
	return out, nil
}

// Create 新增路由并回填数据库生成字段。
func (r *compositeModelRouteRepository) Create(ctx context.Context, route *service.CompositeModelRoute) error {
	if route == nil {
		return service.ErrCompositeRouteNotFound
	}
	created, err := clientFromContext(ctx, r.client).CompositeModelRoute.Create().
		SetGroupID(route.GroupID).SetPublicModel(route.PublicModel).SetMatchType(route.MatchType).
		SetTargetPlatform(route.TargetPlatform).SetUpstreamModel(route.UpstreamModel).SetEndpoint(route.Endpoint).
		SetPriority(route.Priority).SetEnabled(route.Enabled).SetNotes(route.Notes).Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrCompositeRouteExists)
	}
	*route = *compositeModelRouteEntityToService(created)
	return nil
}

// Update 更新已有路由的可编辑字段。
func (r *compositeModelRouteRepository) Update(ctx context.Context, route *service.CompositeModelRoute) error {
	if route == nil {
		return service.ErrCompositeRouteNotFound
	}
	updated, err := clientFromContext(ctx, r.client).CompositeModelRoute.UpdateOneID(route.ID).
		SetPublicModel(route.PublicModel).SetMatchType(route.MatchType).SetTargetPlatform(route.TargetPlatform).
		SetUpstreamModel(route.UpstreamModel).SetEndpoint(route.Endpoint).SetPriority(route.Priority).
		SetEnabled(route.Enabled).SetNotes(route.Notes).Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrCompositeRouteNotFound, service.ErrCompositeRouteExists)
	}
	*route = *compositeModelRouteEntityToService(updated)
	return nil
}

// Delete 软删除指定路由。
func (r *compositeModelRouteRepository) Delete(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).CompositeModelRoute.DeleteOneID(id).Exec(ctx)
	return translatePersistenceError(err, service.ErrCompositeRouteNotFound, nil)
}

// DeleteByGroup 软删除指定分组的全部路由。
func (r *compositeModelRouteRepository) DeleteByGroup(ctx context.Context, groupID int64) error {
	_, err := clientFromContext(ctx, r.client).CompositeModelRoute.Delete().Where(compositemodelroute.GroupIDEQ(groupID)).Exec(ctx)
	return err
}

// compositeModelRouteEntityToService 将 Ent 实体转换为服务层模型。
func compositeModelRouteEntityToService(row *dbent.CompositeModelRoute) *service.CompositeModelRoute {
	if row == nil {
		return nil
	}
	return &service.CompositeModelRoute{ID: row.ID, GroupID: row.GroupID, PublicModel: row.PublicModel, MatchType: row.MatchType, TargetPlatform: row.TargetPlatform, UpstreamModel: row.UpstreamModel, Endpoint: row.Endpoint, Priority: row.Priority, Enabled: row.Enabled, Notes: derefString(row.Notes), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

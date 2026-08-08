package repository

import (
	"context"
	"database/sql"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/compositemodelroute"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// newCompositeModelRouteRepoSQLite 创建包含真实 Ent schema 的内存仓储测试环境。
func newCompositeModelRouteRepoSQLite(t *testing.T) (*compositeModelRouteRepository, *dbent.Client) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return &compositeModelRouteRepository{client: client}, client
}

// mustCreateCompositeRouteGroup 创建满足路由外键约束的分组。
func mustCreateCompositeRouteGroup(t *testing.T, ctx context.Context, client *dbent.Client) int64 {
	t.Helper()
	group, err := client.Group.Create().SetName("composite-route-" + t.Name()).SetPlatform(service.PlatformComposite).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	return group.ID
}

func TestCompositeModelRouteRepositoryLifecycleAndOrdering(t *testing.T) {
	repo, client := newCompositeModelRouteRepoSQLite(t)
	ctx := context.Background()
	groupID := mustCreateCompositeRouteGroup(t, ctx, client)

	disabled := &service.CompositeModelRoute{GroupID: groupID, PublicModel: "router/disabled", MatchType: service.CompositeRouteMatchExact, TargetPlatform: service.PlatformGemini, UpstreamModel: "gemini-2.5-pro", Endpoint: service.CompositeRouteEndpointAny, Priority: 1, Enabled: false, Notes: "disabled"}
	highPriority := &service.CompositeModelRoute{GroupID: groupID, PublicModel: "router/gpt", MatchType: service.CompositeRouteMatchPrefix, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: service.CompositeRouteEndpointResponses, Priority: 10, Enabled: true}
	lowPriority := &service.CompositeModelRoute{GroupID: groupID, PublicModel: "router/claude", MatchType: service.CompositeRouteMatchPrefix, TargetPlatform: service.PlatformAnthropic, UpstreamModel: "claude-sonnet-4-6", Endpoint: service.CompositeRouteEndpointAny, Priority: 100, Enabled: true}

	require.NoError(t, repo.Create(ctx, disabled))
	require.NoError(t, repo.Create(ctx, lowPriority))
	require.NoError(t, repo.Create(ctx, highPriority))
	require.NotZero(t, highPriority.ID)
	require.False(t, highPriority.CreatedAt.IsZero())

	enabled, err := repo.ListByGroup(ctx, groupID, false)
	require.NoError(t, err)
	require.Len(t, enabled, 2)
	require.Equal(t, []int64{highPriority.ID, lowPriority.ID}, []int64{enabled[0].ID, enabled[1].ID})

	all, err := repo.ListByGroup(ctx, groupID, true)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, disabled.ID, all[0].ID)

	highPriority.Priority = 200
	highPriority.Notes = "updated"
	require.NoError(t, repo.Update(ctx, highPriority))
	require.Equal(t, "updated", highPriority.Notes)

	require.NoError(t, repo.Delete(ctx, lowPriority.ID))
	remaining, err := repo.ListByGroup(ctx, groupID, true)
	require.NoError(t, err)
	require.Len(t, remaining, 2)

	deleted, err := client.CompositeModelRoute.Query().Where(compositemodelroute.IDEQ(lowPriority.ID)).Only(mixins.SkipSoftDelete(ctx))
	require.NoError(t, err)
	require.NotNil(t, deleted.DeletedAt)
}

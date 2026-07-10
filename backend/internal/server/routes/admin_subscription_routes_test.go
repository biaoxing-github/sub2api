package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionRoutesRegisterRestoreAndExplicitRevoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	registerSubscriptionRoutes(router.Group(""), &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Subscription: &adminhandler.SubscriptionHandler{},
		},
	})

	registered := make(map[string]struct{})
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}

	require.Contains(t, registered, "POST /subscriptions/:id/revoke")
	require.Contains(t, registered, "POST /subscriptions/:id/restore")
	require.Contains(t, registered, "DELETE /subscriptions/:id")
}

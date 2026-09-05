package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// releaseVersionCache 为版本契约测试提供离线更新缓存。
type releaseVersionCache struct{}

func (releaseVersionCache) GetUpdateInfo(context.Context) (string, error) {
	data, err := json.Marshal(map[string]any{"latest": "v0.2.0", "timestamp": time.Now().Unix()})
	return string(data), err
}

func (releaseVersionCache) SetUpdateInfo(context.Context, string, time.Duration) error { return nil }

// TestSystemVersionPreservesLegacyField 验证主版本、镜像版本和旧字段同时提供。
func TestSystemVersionPreservesLegacyField(t *testing.T) {
	svc := service.NewUpdateServiceWithImageVersion(releaseVersionCache{}, nil, "v0.2.0", "v0.2.0.1", "release")
	h := NewSystemHandler(svc, nil)
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/system/version", nil)
	h.GetVersion(c)
	require.Equal(t, 200, r.Code)
	var response struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &response))
	require.Equal(t, "v0.2.0", response.Data["version"])
	require.Equal(t, "v0.2.0", response.Data["current_version"])
	require.Equal(t, "v0.2.0.1", response.Data["image_version"])
}

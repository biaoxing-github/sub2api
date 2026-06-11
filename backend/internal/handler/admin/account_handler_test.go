package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAccountHandler_ManualProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid account ID", func(t *testing.T) {
		handler := &AccountHandler{}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}
		c.Request = httptest.NewRequest("POST", "/api/v1/admin/accounts/invalid/manual-probe", strings.NewReader(`{}`))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ManualProbe(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRequestHeaderTimeoutForBodyUsesContextSizeBuckets(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 60,
			},
		},
	}

	require.Equal(t, 10*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
	require.Equal(t, 15*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("中", 40000)+`"}`)))
	require.Equal(t, 20*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("中", 160000)+`"}`)))
}

func TestOpenAIRequestHeaderTimeoutForBodyRespectsConfiguredCap(t *testing.T) {
	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIRequestHeaderTimeoutSeconds: 12,
			},
		},
	}

	require.Equal(t, 10*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
	require.Equal(t, 12*time.Second, svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"`+strings.Repeat("x", 160000)+`"}`)))
}

func TestOpenAIRequestHeaderTimeoutForBodyCanBeDisabled(t *testing.T) {
	svc := &OpenAIGatewayService{}

	require.Equal(t, time.Duration(0), svc.openAIRequestHeaderTimeoutForBody([]byte(`{"input":"hello"}`)))
}

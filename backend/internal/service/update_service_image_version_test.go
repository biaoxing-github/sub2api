package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageVersionReleaseClientStub struct{}

type imageVersionCacheStub struct{}

func (imageVersionCacheStub) GetUpdateInfo(ctx context.Context) (string, error) {
	return "", errors.New("cache miss")
}

func (imageVersionCacheStub) SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error {
	return nil
}

func (imageVersionReleaseClientStub) FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error) {
	return nil, errors.New("offline")
}

func (imageVersionReleaseClientStub) DownloadFile(ctx context.Context, url, dest string, maxSize int64) error {
	return nil
}

func (imageVersionReleaseClientStub) FetchChecksumFile(ctx context.Context, url string) ([]byte, error) {
	return nil, nil
}

func TestUpdateServiceCheckUpdateExposesImageVersionSeparately(t *testing.T) {
	t.Setenv("SUB2API_IMAGE_VERSION", "v0.1.136.1")
	svc := NewUpdateService(imageVersionCacheStub{}, imageVersionReleaseClientStub{}, "0.1.136", "release")

	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)

	raw, err := json.Marshal(info)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "0.1.136", payload["current_version"])
	require.Equal(t, "v0.1.136.1", payload["image_version"])
}

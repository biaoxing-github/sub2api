//go:build integration

package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *ProxyRepoSuite) TestProxyExpiryFieldsPersistThroughRepository() {
	expiresAt := time.Now().Add(6 * time.Hour).UTC().Truncate(time.Microsecond)
	backup := s.mustCreateProxy(&service.Proxy{
		Name:     "expiry-backup",
		Protocol: "http",
		Host:     "127.0.0.1",
		Port:     18080,
		Status:   service.StatusActive,
	})

	proxy := s.mustCreateProxy(&service.Proxy{
		Name:           "expiry-main",
		Protocol:       "http",
		Host:           "127.0.0.1",
		Port:           18081,
		Status:         service.StatusActive,
		ExpiresAt:      &expiresAt,
		FallbackMode:   service.FallbackModeProxy,
		BackupProxyID:  &backup.ID,
		ExpiryWarnDays: 3,
	})

	got, err := s.repo.GetByID(s.ctx, proxy.ID)
	s.Require().NoError(err)
	s.Require().NotNil(got.ExpiresAt)
	s.Require().True(got.ExpiresAt.Equal(expiresAt))
	s.Require().Equal(service.FallbackModeProxy, got.FallbackMode)
	s.Require().NotNil(got.BackupProxyID)
	s.Require().Equal(backup.ID, *got.BackupProxyID)
	s.Require().Equal(3, got.ExpiryWarnDays)
}

func (s *ProxyRepoSuite) TestListWithFiltersAndAccountCount_SortByExpiry() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	past := now.Add(-24 * time.Hour)
	soon := now.Add(72 * time.Hour)
	later := now.Add(100 * 24 * time.Hour)

	pLater := s.mustCreateProxy(&service.Proxy{Name: "p-later", Protocol: "http", Host: "127.0.0.1", Port: 18082, Status: service.StatusActive, ExpiresAt: &later})
	pNever := s.mustCreateProxy(&service.Proxy{Name: "p-never", Protocol: "http", Host: "127.0.0.1", Port: 18083, Status: service.StatusActive})
	pExpired := s.mustCreateProxy(&service.Proxy{Name: "p-expired", Protocol: "http", Host: "127.0.0.1", Port: 18084, Status: service.StatusActive, ExpiresAt: &past})
	pSoon := s.mustCreateProxy(&service.Proxy{Name: "p-soon", Protocol: "http", Host: "127.0.0.1", Port: 18085, Status: service.StatusActive, ExpiresAt: &soon})

	asc, _, err := s.repo.ListWithFiltersAndAccountCount(s.ctx, pagination.PaginationParams{
		Page: 1, PageSize: 10, SortBy: "expiry", SortOrder: "asc",
	}, "", "", "")
	s.Require().NoError(err)
	s.Require().Equal(
		[]int64{pExpired.ID, pSoon.ID, pLater.ID, pNever.ID},
		[]int64{asc[0].ID, asc[1].ID, asc[2].ID, asc[3].ID},
	)

	desc, _, err := s.repo.ListWithFiltersAndAccountCount(s.ctx, pagination.PaginationParams{
		Page: 1, PageSize: 10, SortBy: "expiry", SortOrder: "desc",
	}, "", "", "")
	s.Require().NoError(err)
	s.Require().Equal(
		[]int64{pNever.ID, pLater.ID, pSoon.ID, pExpired.ID},
		[]int64{desc[0].ID, desc[1].ID, desc[2].ID, desc[3].ID},
	)
}

func (s *ProxyRepoSuite) TestSweepExpiredProxiesMovesAccountsToBackupOrDirect() {
	now := time.Now().UTC().Truncate(time.Microsecond)
	past := now.Add(-time.Hour)
	future := now.Add(24 * time.Hour)

	backup := s.mustCreateProxy(&service.Proxy{Name: "fallback-backup", Protocol: "http", Host: "127.0.0.1", Port: 18086, Status: service.StatusActive, ExpiresAt: &future})
	toProxy := s.mustCreateProxy(&service.Proxy{Name: "fallback-proxy", Protocol: "http", Host: "127.0.0.1", Port: 18087, Status: service.StatusActive, ExpiresAt: &past, FallbackMode: service.FallbackModeProxy, BackupProxyID: &backup.ID})
	toDirect := s.mustCreateProxy(&service.Proxy{Name: "fallback-direct", Protocol: "http", Host: "127.0.0.1", Port: 18088, Status: service.StatusActive, ExpiresAt: &past, FallbackMode: service.FallbackModeDirect})

	proxyAccountID := s.mustInsertAccountWithProxy("proxy-account", toProxy.ID)
	directAccountID := s.mustInsertAccountWithProxy("direct-account", toDirect.ID)

	changed, err := s.repo.SweepExpiredProxies(s.ctx, now)
	s.Require().NoError(err)
	s.Require().Equal(int64(2), changed)

	s.Require().Equal(backup.ID, *s.accountProxyID(proxyAccountID))
	s.Require().Nil(s.accountProxyID(directAccountID))
	s.Require().Equal(toProxy.ID, *s.accountFallbackOriginID(proxyAccountID))
	s.Require().Equal(toDirect.ID, *s.accountFallbackOriginID(directAccountID))

	gotProxy, err := s.repo.GetByID(s.ctx, toProxy.ID)
	s.Require().NoError(err)
	s.Require().Equal(service.StatusExpired, gotProxy.Status)
	gotDirect, err := s.repo.GetByID(s.ctx, toDirect.ID)
	s.Require().NoError(err)
	s.Require().Equal(service.StatusExpired, gotDirect.Status)
}

func (s *ProxyRepoSuite) mustInsertAccountWithProxy(name string, proxyID int64) int64 {
	s.T().Helper()

	var id int64
	err := scanSingleRow(s.ctx, s.tx, `
		INSERT INTO accounts (name, platform, type, proxy_id, credentials, extra, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '{}', '{}', $5, NOW(), NOW())
		RETURNING id`,
		[]any{name, service.PlatformAnthropic, service.AccountTypeOAuth, proxyID, service.StatusActive}, &id)
	s.Require().NoError(err)
	return id
}

func (s *ProxyRepoSuite) accountProxyID(accountID int64) *int64 {
	s.T().Helper()

	var proxyID *int64
	err := scanSingleRow(s.ctx, s.tx, `SELECT proxy_id FROM accounts WHERE id = $1`, []any{accountID}, &proxyID)
	s.Require().NoError(err)
	return proxyID
}

func (s *ProxyRepoSuite) accountFallbackOriginID(accountID int64) *int64 {
	s.T().Helper()

	var originID *int64
	err := scanSingleRow(s.ctx, s.tx, `SELECT proxy_fallback_origin_id FROM accounts WHERE id = $1`, []any{accountID}, &originID)
	s.Require().NoError(err)
	return originID
}

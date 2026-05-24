//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "", "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByTotalAccountCostDesc() {
	low := mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-cost"})
	high := mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-cost"})

	insertAccountUsageLog(s.T(), s.ctx, s.client, s.repo.sql, high.ID, 2.5, 3)
	insertAccountUsageLog(s.T(), s.ctx, s.client, s.repo.sql, low.ID, 1, 1)

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "total_account_cost",
		SortOrder: "desc",
	}, "", "", "", "", 0, "", "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-cost", accounts[0].Name)
	s.Require().Equal(7.5, accounts[0].TotalAccountCost)
	s.Require().Equal(int64(1), accounts[0].TotalRequests)
	s.Require().Equal("low-cost", accounts[1].Name)
	s.Require().Equal(1.0, accounts[1].TotalAccountCost)
	s.Require().Equal(int64(1), accounts[1].TotalRequests)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByTotalRequestsDesc() {
	one := mustCreateAccount(s.T(), s.client, &service.Account{Name: "one-request"})
	two := mustCreateAccount(s.T(), s.client, &service.Account{Name: "two-requests"})

	insertAccountUsageLog(s.T(), s.ctx, s.client, s.repo.sql, one.ID, 10, 1)
	insertAccountUsageLog(s.T(), s.ctx, s.client, s.repo.sql, two.ID, 1, 1)
	insertAccountUsageLog(s.T(), s.ctx, s.client, s.repo.sql, two.ID, 1, 1)

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "total_requests",
		SortOrder: "desc",
	}, "", "", "", "", 0, "", "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("two-requests", accounts[0].Name)
	s.Require().Equal(int64(2), accounts[0].TotalRequests)
	s.Require().Equal("one-request", accounts[1].Name)
	s.Require().Equal(int64(1), accounts[1].TotalRequests)
}

func insertAccountUsageLog(t *testing.T, ctx context.Context, client *dbent.Client, exec sqlExecutor, accountID int64, totalCost, accountRateMultiplier float64) {
	t.Helper()
	user := mustCreateUser(t, client, &service.User{})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID})
	_, err := exec.ExecContext(ctx, `
		INSERT INTO usage_logs (
			user_id,
			api_key_id,
			account_id,
			model,
			input_tokens,
			output_tokens,
			total_cost,
			actual_cost,
			account_rate_multiplier,
			created_at
		)
		VALUES ($1, $2, $3, 'test-model', 0, 0, $4, $4, $5, NOW())
	`, user.ID, apiKey.ID, accountID, totalCost, accountRateMultiplier)
	require.NoError(t, err)
}

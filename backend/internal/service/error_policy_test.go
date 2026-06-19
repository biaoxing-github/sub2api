//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestCheckErrorPolicy — 6 table-driven cases for the pure logic function
// ---------------------------------------------------------------------------

func TestCheckErrorPolicy(t *testing.T) {
	tests := []struct {
		name       string
		account    *Account
		statusCode int
		body       []byte
		expected   ErrorPolicyResult
	}{
		{
			name: "no_policy_oauth_returns_none",
			account: &Account{
				ID:       1,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				// no custom error codes, no temp rules
			},
			statusCode: 500,
			body:       []byte(`"error"`),
			expected:   ErrorPolicyNone,
		},
		{
			name: "custom_error_codes_hit_returns_matched",
			account: &Account{
				ID:       2,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429), float64(500)},
				},
			},
			statusCode: 500,
			body:       []byte(`"error"`),
			expected:   ErrorPolicyMatched,
		},
		{
			name: "custom_error_codes_miss_returns_skipped",
			account: &Account{
				ID:       3,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429), float64(500)},
				},
			},
			statusCode: 503,
			body:       []byte(`"error"`),
			expected:   ErrorPolicySkipped,
		},
		{
			name: "temp_unschedulable_hit_returns_temp_unscheduled",
			account: &Account{
				ID:       4,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`overloaded service`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			name: "temp_unschedulable_401_first_hit_returns_temp_unscheduled",
			account: &Account{
				ID:       14,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(401),
							"keywords":         []any{"unauthorized"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			// Antigravity 401 不走升级逻辑（由 applyErrorPolicy 的 temp_unschedulable_rules 自行控制），
			// second hit 仍然返回 TempUnscheduled。
			name: "temp_unschedulable_401_second_hit_antigravity_stays_temp",
			account: &Account{
				ID:                      15,
				Type:                    AccountTypeOAuth,
				Platform:                PlatformAntigravity,
				TempUnschedulableReason: `{"status_code":401,"until_unix":1735689600}`,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(401),
							"keywords":         []any{"unauthorized"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			name: "temp_unschedulable_body_miss_returns_none",
			account: &Account{
				ID:       5,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`random msg`),
			expected:   ErrorPolicyNone,
		},
		{
			name: "custom_error_codes_override_temp_unschedulable",
			account: &Account{
				ID:       6,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(503)},
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`overloaded`),
			expected:   ErrorPolicyMatched, // custom codes take precedence
		},
		{
			name: "pool_mode_custom_error_codes_hit_returns_matched",
			account: &Account{
				ID:       7,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":                  true,
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(401), float64(403)},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyMatched,
		},
		{
			name: "pool_mode_without_custom_error_codes_returns_skipped",
			account: &Account{
				ID:       8,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode": true,
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicySkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

			result := svc.CheckErrorPolicy(context.Background(), tt.account, tt.statusCode, tt.body)
			require.Equal(t, tt.expected, result, "unexpected ErrorPolicyResult")
		})
	}
}

func TestHandleUpstreamError_PoolModeCustomErrorCodesOverride(t *testing.T) {
	t.Run("pool_mode_without_custom_error_codes_still_skips", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       30,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"pool_mode": true,
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})

	t.Run("pool_mode_with_custom_error_codes_uses_local_error_policy", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       31,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"pool_mode":                  true,
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(401)},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})
}

func TestAccountGetErrorHandlingRules_NormalizesUnifiedSchema(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"error_handling_rules": []any{
				map[string]any{
					"enabled":  true,
					"name":     "nested match",
					"priority": float64(20),
					"action":   "rate_limited",
					"match": map[string]any{
						"status_codes": []any{float64(429), "429", float64(200), "bad"},
						"error_codes":  []any{"insufficient_quota", "INSUFFICIENT_QUOTA", ""},
						"error_types":  []any{"rate_limit_error"},
						"keywords":     []any{"quota", "quota", "额度"},
					},
					"reset_strategy": map[string]any{
						"type":              "weekly",
						"weekly_reset_day":  float64(1),
						"weekly_reset_hour": float64(3),
					},
				},
				map[string]any{
					"enabled":         true,
					"name":            "temporary",
					"priority":        float64(10),
					"action":          "temp_unschedulable",
					"status_codes":    []any{float64(503)},
					"durationMinutes": float64(15),
				},
				map[string]any{
					"enabled":      true,
					"name":         "invalid success matcher",
					"priority":     float64(1),
					"action":       "retry_next",
					"status_codes": []any{float64(200)},
				},
			},
		},
	}

	rules := account.GetErrorHandlingRules()

	require.Len(t, rules, 2)
	require.Equal(t, "temporary", rules[0].Name)
	require.Equal(t, AccountErrorHandlingActionTempUnschedulable, rules[0].Action)
	require.Equal(t, []int{503}, rules[0].StatusCodes)
	require.Equal(t, 15, rules[0].DurationMinutes)

	require.Equal(t, "nested match", rules[1].Name)
	require.Equal(t, AccountErrorHandlingActionRateLimited, rules[1].Action)
	require.Equal(t, []int{429}, rules[1].StatusCodes)
	require.Equal(t, []string{"insufficient_quota"}, rules[1].ErrorCodes)
	require.Equal(t, []string{"rate_limit_error"}, rules[1].ErrorTypes)
	require.Equal(t, []string{"quota", "额度"}, rules[1].Keywords)
	require.Equal(t, AccountErrorHandlingResetWeekly, rules[1].ResetStrategy)
	require.Equal(t, 1, rules[1].WeeklyResetDay)
	require.Equal(t, 3, rules[1].WeeklyResetHour)
}

func TestCheckErrorPolicy_UnifiedErrorHandlingRules(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		status   int
		body     []byte
		expected ErrorPolicyResult
	}{
		{
			name: "retry_next_matches_error_code",
			account: &Account{
				ID:       40,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"error_handling_rules": []any{
						map[string]any{
							"enabled":      true,
							"name":         "quota retry",
							"priority":     float64(10),
							"action":       "retry_next",
							"status_codes": []any{float64(429)},
							"error_codes":  []any{"insufficient_quota"},
						},
					},
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(500)},
				},
			},
			status:   429,
			body:     []byte(`{"error":{"code":"insufficient_quota","type":"rate_limit_error","message":"quota exceeded"}}`),
			expected: ErrorPolicyRetryNext,
		},
		{
			name: "disabled_rule_is_ignored_and_falls_back_to_old_custom_codes",
			account: &Account{
				ID:       41,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"error_handling_rules": []any{
						map[string]any{
							"enabled":      false,
							"name":         "disabled retry",
							"priority":     float64(1),
							"action":       "retry_next",
							"status_codes": []any{float64(500)},
						},
					},
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(500)},
				},
			},
			status:   500,
			body:     []byte(`{"error":{"message":"server error"}}`),
			expected: ErrorPolicyMatched,
		},
		{
			name: "unified_rules_take_precedence_over_old_temp_rules",
			account: &Account{
				ID:       42,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"error_handling_rules": []any{
						map[string]any{
							"enabled":      true,
							"name":         "retry 503",
							"priority":     float64(1),
							"action":       "retry_next",
							"status_codes": []any{float64(503)},
							"keywords":     []any{"overloaded"},
						},
					},
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			status:   503,
			body:     []byte(`overloaded`),
			expected: ErrorPolicyRetryNext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

			result := svc.CheckErrorPolicy(context.Background(), tt.account, tt.status, tt.body)

			require.Equal(t, tt.expected, result)
			require.Equal(t, 0, repo.tempCalls)
			require.Equal(t, 0, repo.setErrCalls)
			require.Equal(t, 0, repo.rateCalls)
		})
	}
}

func TestHandleUpstreamError_UnifiedErrorHandlingRules(t *testing.T) {
	t.Run("temp_unschedulable_writes_temp_state", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       50,
			Type:     AccountTypeOAuth,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"error_handling_rules": []any{
					map[string]any{
						"enabled":         true,
						"name":            "capacity cooldown",
						"priority":        float64(1),
						"action":          "temp_unschedulable",
						"status_codes":    []any{float64(529)},
						"keywords":        []any{"overloaded"},
						"durationMinutes": float64(15),
					},
				},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 529, http.Header{}, []byte(`{"error":{"message":"server overloaded"}}`))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.tempCalls)
		require.Equal(t, int64(50), repo.lastTempID)
		require.True(t, repo.lastTempUntil.After(time.Now().Add(14*time.Minute)))
		require.Contains(t, repo.lastTempReason, "capacity cooldown")
		require.Equal(t, 0, repo.rateCalls)
		require.Equal(t, 0, repo.setErrCalls)
	})

	t.Run("rate_limited_duration_writes_rate_limit", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       51,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"error_handling_rules": []any{
					map[string]any{
						"enabled":        true,
						"name":           "quota window",
						"priority":       float64(1),
						"action":         "rate_limited",
						"status_codes":   []any{float64(429)},
						"error_types":    []any{"rate_limit_error"},
						"reset_strategy": "duration",
						"duration_hours": float64(2),
					},
				},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))

		require.False(t, shouldDisable)
		require.Equal(t, 1, repo.rateCalls)
		require.Equal(t, int64(51), repo.lastRateID)
		require.True(t, repo.lastRateUntil.After(time.Now().Add(119*time.Minute)))
		require.Equal(t, 0, repo.tempCalls)
		require.Equal(t, 0, repo.setErrCalls)
	})

	t.Run("error_disabled_writes_error", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       52,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"error_handling_rules": []any{
					map[string]any{
						"enabled":      true,
						"name":         "invalid key",
						"priority":     float64(1),
						"action":       "error_disabled",
						"status_codes": []any{float64(401)},
						"error_codes":  []any{"invalid_api_key"},
					},
				},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte(`{"error":{"code":"invalid_api_key","message":"bad key"}}`))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrCalls)
		require.Contains(t, repo.lastErrorMsg, "invalid key")
		require.Equal(t, 0, repo.tempCalls)
		require.Equal(t, 0, repo.rateCalls)
	})

	t.Run("retry_next_has_no_state_side_effect", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       53,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"error_handling_rules": []any{
					map[string]any{
						"enabled":      true,
						"name":         "try another account",
						"priority":     float64(1),
						"action":       "retry_next",
						"status_codes": []any{float64(500)},
						"keywords":     []any{"internal"},
					},
				},
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(500)},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 500, http.Header{}, []byte(`internal server error`))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.tempCalls)
		require.Equal(t, 0, repo.rateCalls)
		require.Equal(t, 0, repo.setErrCalls)
	})

	t.Run("openai_5xx_temp_unschedulable_rules_override_state_skip", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       54,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"temp_unschedulable_enabled": true,
				"temp_unschedulable_rules": []any{
					map[string]any{
						"error_code":       float64(http.StatusBadGateway),
						"keywords":         []any{"bad gateway"},
						"duration_minutes": float64(10),
						"description":      "upstream gateway cooldown",
					},
				},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusBadGateway, http.Header{}, []byte(`bad gateway from upstream`))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.tempCalls)
		require.Equal(t, int64(54), repo.lastTempID)
		require.True(t, repo.lastTempUntil.After(time.Now().Add(9*time.Minute)))
		require.Contains(t, repo.lastTempReason, `"status_code":502`)
		require.Contains(t, repo.lastTempReason, `"matched_keyword":"bad gateway"`)
		require.Equal(t, 0, repo.rateCalls)
		require.Equal(t, 0, repo.setErrCalls)
	})
}

func TestResolveUnifiedRateLimitResetAt(t *testing.T) {
	now := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)

	t.Run("daily_same_day_when_future_hour", func(t *testing.T) {
		resetAt, ok := resolveUnifiedRateLimitResetAt(AccountErrorHandlingRule{
			Action:         AccountErrorHandlingActionRateLimited,
			ResetStrategy:  AccountErrorHandlingResetDaily,
			DailyResetHour: 12,
		}, now)

		require.True(t, ok)
		require.Equal(t, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), resetAt)
	})

	t.Run("daily_next_day_when_hour_passed", func(t *testing.T) {
		resetAt, ok := resolveUnifiedRateLimitResetAt(AccountErrorHandlingRule{
			Action:         AccountErrorHandlingActionRateLimited,
			ResetStrategy:  AccountErrorHandlingResetDaily,
			DailyResetHour: 9,
		}, now)

		require.True(t, ok)
		require.Equal(t, time.Date(2026, 6, 9, 9, 0, 0, 0, time.UTC), resetAt)
	})

	t.Run("weekly_same_week_when_future_slot", func(t *testing.T) {
		resetAt, ok := resolveUnifiedRateLimitResetAt(AccountErrorHandlingRule{
			Action:          AccountErrorHandlingActionRateLimited,
			ResetStrategy:   AccountErrorHandlingResetWeekly,
			WeeklyResetDay:  int(time.Monday),
			WeeklyResetHour: 12,
		}, now)

		require.True(t, ok)
		require.Equal(t, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), resetAt)
	})

	t.Run("weekly_next_week_when_slot_passed", func(t *testing.T) {
		resetAt, ok := resolveUnifiedRateLimitResetAt(AccountErrorHandlingRule{
			Action:          AccountErrorHandlingActionRateLimited,
			ResetStrategy:   AccountErrorHandlingResetWeekly,
			WeeklyResetDay:  int(time.Monday),
			WeeklyResetHour: 9,
		}, now)

		require.True(t, ok)
		require.Equal(t, time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC), resetAt)
	})
}

// ---------------------------------------------------------------------------
// TestApplyErrorPolicy — 4 table-driven cases for the wrapper method
// ---------------------------------------------------------------------------

func TestApplyErrorPolicy(t *testing.T) {
	tests := []struct {
		name              string
		account           *Account
		statusCode        int
		body              []byte
		expectedHandled   bool
		expectedStatus    int  // expected outStatus
		expectedSwitchErr bool // expect *AntigravityAccountSwitchError
		handleErrorCalls  int
	}{
		{
			name: "none_not_handled",
			account: &Account{
				ID:       10,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  false,
			expectedStatus:   500, // passthrough
			handleErrorCalls: 0,
		},
		{
			name: "skipped_handled_no_handleError",
			account: &Account{
				ID:       11,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429)},
				},
			},
			statusCode:       500, // not in custom codes
			body:             []byte(`"error"`),
			expectedHandled:  true,
			expectedStatus:   http.StatusInternalServerError, // skipped → 500
			handleErrorCalls: 0,
		},
		{
			name: "matched_handled_calls_handleError",
			account: &Account{
				ID:       12,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(500)},
				},
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  true,
			expectedStatus:   500, // matched → original status
			handleErrorCalls: 1,
		},
		{
			name: "temp_unscheduled_returns_switch_error",
			account: &Account{
				ID:       13,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode:        503,
			body:              []byte(`overloaded`),
			expectedHandled:   true,
			expectedStatus:    503, // temp_unscheduled → original status
			expectedSwitchErr: true,
			handleErrorCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			svc := &AntigravityGatewayService{
				rateLimitService: rlSvc,
			}

			var handleErrorCount int
			p := antigravityRetryLoopParams{
				ctx:     context.Background(),
				prefix:  "[test]",
				account: tt.account,
				handleError: func(ctx context.Context, prefix string, account *Account, statusCode int, headers http.Header, body []byte, requestedModel string, groupID int64, sessionHash string, isStickySession bool) *handleModelRateLimitResult {
					handleErrorCount++
					return nil
				},
				isStickySession: true,
			}

			handled, outStatus, retErr := svc.applyErrorPolicy(p, tt.statusCode, http.Header{}, tt.body)

			require.Equal(t, tt.expectedHandled, handled, "handled mismatch")
			require.Equal(t, tt.expectedStatus, outStatus, "outStatus mismatch")
			require.Equal(t, tt.handleErrorCalls, handleErrorCount, "handleError call count mismatch")

			if tt.expectedSwitchErr {
				var switchErr *AntigravityAccountSwitchError
				require.ErrorAs(t, retErr, &switchErr)
				require.Equal(t, tt.account.ID, switchErr.OriginalAccountID)
			} else {
				require.NoError(t, retErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// errorPolicyRepoStub — minimal AccountRepository stub for error policy tests
// ---------------------------------------------------------------------------

type errorPolicyRepoStub struct {
	mockAccountRepoForGemini
	tempCalls      int
	lastTempID     int64
	lastTempUntil  time.Time
	lastTempReason string
	rateCalls      int
	lastRateID     int64
	lastRateUntil  time.Time
	setErrCalls    int
	lastErrorMsg   string
}

func (r *errorPolicyRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempID = id
	r.lastTempUntil = until
	r.lastTempReason = reason
	return nil
}

func (r *errorPolicyRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.rateCalls++
	r.lastRateID = id
	r.lastRateUntil = resetAt
	return nil
}

func (r *errorPolicyRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

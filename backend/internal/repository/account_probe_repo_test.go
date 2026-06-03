package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type accountProbeEvidenceJSONArg struct {
	key string
}

func (a accountProbeEvidenceJSONArg) Match(value driver.Value) bool {
	raw, ok := value.(string)
	if !ok {
		return false
	}
	var evidence []service.AccountProbeValidationEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return false
	}
	return len(evidence) == 1 && evidence[0].Key == a.key
}

func TestAccountProbeRepositoryExpireStaleRuns(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("UPDATE account_probe_runs").
		WithArgs("running", "failed", "probe run timed out before completion", "体检任务超时未完成，已自动收尾", "900 seconds").
		WillReturnResult(sqlmock.NewResult(0, 3))

	repo := NewAccountProbeRepository(db)
	err = repo.ExpireStaleAccountProbeRuns(context.Background(), 15*time.Minute)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountProbeRepositoryDeleteReportRunsSkipsRunning(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE account_probe_runs").
		WithArgs("running", "failed", "probe run timed out before completion", "体检任务超时未完成，已自动收尾", "720 seconds").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM account_probe_runs").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("DELETE FROM account_probe_samples").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM account_probe_runs").
		WithArgs(sqlmock.AnyArg(), "running").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repo := NewAccountProbeRepository(db)
	result, err := repo.DeleteAccountProbeReportRuns(context.Background(), []int64{91, 92, 93})

	require.NoError(t, err)
	require.Equal(t, 3, result.RequestedCount)
	require.Equal(t, 2, result.DeletedCount)
	require.Equal(t, 1, result.SkippedRunningCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountProbeRepositorySaveSampleStoresValidationEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO account_probe_samples").
		WithArgs(
			int64(9), 1, "model_validation", "模型验证：JSON 算术", service.AccountProbeSampleSuccess, "gpt-test",
			"fp", "sk-...test", "https://example.test/v1/responses", 200,
			123, sqlmock.AnyArg(),
			18, 8, 26,
			`{"sum":83,"code":"BETA"}`, accountProbeEvidenceJSONArg{key: "json_arithmetic"},
			"", "", sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewAccountProbeRepository(db)
	err = repo.SaveAccountProbeSample(context.Background(), service.AccountProbeSample{
		RunID:             9,
		RequestIndex:      1,
		Type:              "model_validation",
		Label:             "模型验证：JSON 算术",
		Status:            service.AccountProbeSampleSuccess,
		Model:             "gpt-test",
		APIKeyFingerprint: "fp",
		APIKeyMasked:      "sk-...test",
		UpstreamEndpoint:  "https://example.test/v1/responses",
		HTTPStatus:        200,
		DurationMillis:    123,
		InputTokens:       18,
		OutputTokens:      8,
		TotalTokens:       26,
		OutputText:        `{"sum":83,"code":"BETA"}`,
		ValidationEvidence: []service.AccountProbeValidationEvidence{{
			Key:      "json_arithmetic",
			Label:    "JSON 算术",
			Expected: `{"sum":83,"code":"BETA"}`,
			Observed: `{"sum":83,"code":"BETA"}`,
			Passed:   true,
			Score:    10,
			MaxScore: 10,
		}},
		CreatedAt: time.Now(),
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountProbeRepositoryListSamplesLoadsValidationEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	firstToken := sql.NullInt64{Int64: 88, Valid: true}
	evidence, err := json.Marshal([]service.AccountProbeValidationEvidence{{
		Key:      "json_arithmetic",
		Label:    "JSON 算术",
		Expected: `{"sum":83,"code":"BETA"}`,
		Observed: `{"sum":83,"code":"BETA"}`,
		Passed:   true,
		Score:    10,
		MaxScore: 10,
	}})
	require.NoError(t, err)
	createdAt := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "run_id", "request_index", "sample_type", "label", "status", "model",
		"api_key_fingerprint", "api_key_masked", "upstream_endpoint", "http_status",
		"duration_ms", "first_token_ms",
		"input_tokens", "output_tokens", "total_tokens",
		"output_text", "validation_evidence",
		"error_code", "error_message", "created_at",
	}).AddRow(
		int64(1), int64(9), 1, "model_validation", "模型验证：JSON 算术", service.AccountProbeSampleSuccess, "gpt-test",
		"fp", "sk-...test", "https://example.test/v1/responses", 200,
		123, firstToken,
		18, 8, 26,
		`{"sum":83,"code":"BETA"}`, evidence,
		"", "", createdAt,
	)
	mock.ExpectQuery("SELECT id, run_id, request_index").WithArgs(int64(9)).WillReturnRows(rows)

	repo := NewAccountProbeRepository(db)
	samples, err := repo.ListAccountProbeSamples(context.Background(), 9)

	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, `{"sum":83,"code":"BETA"}`, samples[0].OutputText)
	require.NotNil(t, samples[0].FirstTokenMillis)
	require.Equal(t, 88, *samples[0].FirstTokenMillis)
	require.Len(t, samples[0].ValidationEvidence, 1)
	require.True(t, samples[0].ValidationEvidence[0].Passed)
	require.NoError(t, mock.ExpectationsWereMet())
}

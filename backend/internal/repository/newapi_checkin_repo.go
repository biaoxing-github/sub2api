package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type newAPICheckinRepository struct {
	db *sql.DB
}

// NewAPICheckinRepository 创建 NewApi 签到 SQL 仓储。
func NewAPICheckinRepository(db *sql.DB) service.NewAPICheckinRepository {
	if db == nil {
		return nil
	}
	return &newAPICheckinRepository{db: db}
}

func (r *newAPICheckinRepository) StorageLabel() string {
	return "sql:newapi-checkin"
}

func (r *newAPICheckinRepository) LoadConfig(ctx context.Context) (service.NewAPICheckinConfig, error) {
	cfg := service.NewAPICheckinConfig{
		DefaultCheckinPath: "/api/user/checkin",
		Sites:              []service.NewAPICheckinSite{},
	}
	var notify bool
	err := r.db.QueryRowContext(ctx, `
SELECT default_checkin_path, delay_between_checkins_sec, route_switch_wait_sec, notify_feishu
FROM newapi_checkin_settings
WHERE id = 1`).Scan(&cfg.DefaultCheckinPath, &cfg.DelayBetweenCheckinsSec, &cfg.RouteSwitchWaitSec, &notify)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cfg, err
	}
	cfg.NotifyFeishu = notify

	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, enabled, disabled_reason, background_checkin_enabled, base_url, checkin_path,
       site_status_ok, site_status_message, quota_display_type, quota_per_unit, custom_currency_symbol
FROM newapi_checkin_sites
ORDER BY id ASC`)
	if err != nil {
		return cfg, err
	}
	defer rows.Close()

	siteIndex := map[int64]int{}
	for rows.Next() {
		var id int64
		var site service.NewAPICheckinSite
		if err := rows.Scan(
			&id, &site.Name, &site.Enabled, &site.DisabledReason, &site.BackgroundCheckinEnabled, &site.BaseURL, &site.CheckinPath,
			&site.SiteStatus.OK, &site.SiteStatus.Message, &site.SiteStatus.QuotaDisplayType, &site.SiteStatus.QuotaPerUnit, &site.SiteStatus.CustomCurrencySymbol,
		); err != nil {
			return cfg, err
		}
		site.Accounts = []service.NewAPICheckinAccount{}
		siteIndex[id] = len(cfg.Sites)
		cfg.Sites = append(cfg.Sites, site)
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}

	accountRows, err := r.db.QueryContext(ctx, `
SELECT site_id, name, username, display_name, user_id, access_key, ip_profile, enabled, disabled_reason
FROM newapi_checkin_accounts
ORDER BY site_id ASC, id ASC`)
	if err != nil {
		return cfg, err
	}
	defer accountRows.Close()
	for accountRows.Next() {
		var siteID int64
		var account service.NewAPICheckinAccount
		if err := accountRows.Scan(
			&siteID, &account.Name, &account.Username, &account.DisplayName, &account.UserID, &account.AccessKey,
			&account.IPProfile, &account.Enabled, &account.DisabledReason,
		); err != nil {
			return cfg, err
		}
		if idx, ok := siteIndex[siteID]; ok {
			cfg.Sites[idx].Accounts = append(cfg.Sites[idx].Accounts, account)
		}
	}
	return cfg, accountRows.Err()
}

func (r *newAPICheckinRepository) SaveConfig(ctx context.Context, cfg service.NewAPICheckinConfig) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if cfg.DefaultCheckinPath == "" {
		cfg.DefaultCheckinPath = "/api/user/checkin"
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_settings (id, default_checkin_path, delay_between_checkins_sec, route_switch_wait_sec, notify_feishu, updated_at)
VALUES (1, $1, $2, $3, $4, NOW())
ON CONFLICT (id) DO UPDATE SET
  default_checkin_path = EXCLUDED.default_checkin_path,
  delay_between_checkins_sec = EXCLUDED.delay_between_checkins_sec,
  route_switch_wait_sec = EXCLUDED.route_switch_wait_sec,
  notify_feishu = EXCLUDED.notify_feishu,
  updated_at = NOW()`,
		cfg.DefaultCheckinPath, cfg.DelayBetweenCheckinsSec, cfg.RouteSwitchWaitSec, cfg.NotifyFeishu,
	); err != nil {
		return err
	}

	siteNames := make([]string, 0, len(cfg.Sites))
	for _, site := range cfg.Sites {
		status := normalizeRepositorySiteStatus(site.SiteStatus)
		var siteID int64
		if err := tx.QueryRowContext(ctx, `
INSERT INTO newapi_checkin_sites (
  name, enabled, disabled_reason, background_checkin_enabled, base_url, checkin_path,
  site_status_ok, site_status_message, quota_display_type, quota_per_unit, custom_currency_symbol, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
ON CONFLICT (name) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  disabled_reason = EXCLUDED.disabled_reason,
  background_checkin_enabled = EXCLUDED.background_checkin_enabled,
  base_url = EXCLUDED.base_url,
  checkin_path = EXCLUDED.checkin_path,
  site_status_ok = EXCLUDED.site_status_ok,
  site_status_message = EXCLUDED.site_status_message,
  quota_display_type = EXCLUDED.quota_display_type,
  quota_per_unit = EXCLUDED.quota_per_unit,
  custom_currency_symbol = EXCLUDED.custom_currency_symbol,
  updated_at = NOW()
RETURNING id`,
			site.Name, site.Enabled, site.DisabledReason, site.BackgroundCheckinEnabled, site.BaseURL, site.CheckinPath,
			status.OK, status.Message, status.QuotaDisplayType, status.QuotaPerUnit, status.CustomCurrencySymbol,
		).Scan(&siteID); err != nil {
			return err
		}
		siteNames = append(siteNames, site.Name)

		userIDs := make([]string, 0, len(site.Accounts))
		for _, account := range site.Accounts {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_accounts (
  site_id, name, username, display_name, user_id, access_key, ip_profile, enabled, disabled_reason, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
ON CONFLICT (site_id, user_id) DO UPDATE SET
  name = EXCLUDED.name,
  username = EXCLUDED.username,
  display_name = EXCLUDED.display_name,
  access_key = EXCLUDED.access_key,
  ip_profile = EXCLUDED.ip_profile,
  enabled = EXCLUDED.enabled,
  disabled_reason = EXCLUDED.disabled_reason,
  updated_at = NOW()`,
				siteID, account.Name, account.Username, account.DisplayName, account.UserID, account.AccessKey,
				account.IPProfile, account.Enabled, account.DisabledReason,
			); err != nil {
				return err
			}
			userIDs = append(userIDs, account.UserID)
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM newapi_checkin_accounts
WHERE site_id = $1 AND NOT (user_id = ANY($2))`, siteID, pq.Array(userIDs)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM newapi_checkin_sites
WHERE NOT (name = ANY($1))`, pq.Array(siteNames)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *newAPICheckinRepository) LoadLatestReport(ctx context.Context) (service.NewAPICheckinReport, error) {
	var report service.NewAPICheckinReport
	var runID int64
	err := r.db.QueryRowContext(ctx, `
SELECT id, started_at, ended_at, source, site_count, task_count, success_count, already_done_count, failed_count,
       quota_awarded_total, quota_awarded_display, quota_display_symbol, summary_text
FROM newapi_checkin_runs
ORDER BY created_at DESC, id DESC
LIMIT 1`).Scan(
		&runID, &report.StartedAt, &report.EndedAt, &report.Source, &report.SiteCount, &report.TaskCount,
		&report.SuccessCount, &report.AlreadyDoneCount, &report.FailedCount, &report.QuotaAwardedTotal,
		&report.QuotaAwardedDisplay, &report.QuotaDisplaySymbol, &report.SummaryText,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return report, nil
	}
	if err != nil {
		return report, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT site, account, user_id, ip_profile, ok, success, message, checkin_date,
       quota_awarded, quota_awarded_display, quota_awarded_display_value,
       remaining_quota, remaining_quota_display, used_quota, used_quota_display,
       username, display_name, status, checkin_status, checkin_status_tone
FROM newapi_checkin_run_results
WHERE run_id = $1
ORDER BY order_index ASC, id ASC`, runID)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		row, err := scanNewAPICheckinRunResult(rows)
		if err != nil {
			return report, err
		}
		report.AccountResults = append(report.AccountResults, row)
	}
	return report, rows.Err()
}

func (r *newAPICheckinRepository) SaveLatestReport(ctx context.Context, report service.NewAPICheckinReport) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM newapi_checkin_runs`); err != nil {
		return err
	}
	var runID int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO newapi_checkin_runs (
  started_at, ended_at, source, site_count, task_count, success_count, already_done_count, failed_count,
  quota_awarded_total, quota_awarded_display, quota_display_symbol, summary_text
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id`,
		report.StartedAt, report.EndedAt, report.Source, report.SiteCount, report.TaskCount, report.SuccessCount,
		report.AlreadyDoneCount, report.FailedCount, report.QuotaAwardedTotal, report.QuotaAwardedDisplay,
		report.QuotaDisplaySymbol, report.SummaryText,
	).Scan(&runID); err != nil {
		return err
	}
	for idx, row := range report.AccountResults {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_run_results (
  run_id, order_index, site, account, user_id, ip_profile, ok, success, message, checkin_date,
  quota_awarded, quota_awarded_display, quota_awarded_display_value,
  remaining_quota, remaining_quota_display, used_quota, used_quota_display,
  username, display_name, status, checkin_status, checkin_status_tone
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
			runID, idx, row.Site, row.Account, row.UserID, row.IPProfile, row.OK, row.Success, row.Message, row.CheckinDate,
			nullableInt64Ptr(row.QuotaAwarded), row.QuotaAwardedDisplay, row.QuotaAwardedDisplayValue,
			nullableInt64Ptr(row.RemainingQuota), row.RemainingQuotaDisplay, nullableInt64Ptr(row.UsedQuota), row.UsedQuotaDisplay,
			row.Username, row.DisplayName, row.Status, row.CheckinStatus, row.CheckinStatusTone,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *newAPICheckinRepository) LoadBalanceCache(ctx context.Context) (service.NewAPICheckinBalancePayload, error) {
	payload := service.NewAPICheckinBalancePayload{
		Source:       "sql-cache",
		SiteStatuses: map[string]service.NewAPICheckinSiteStatus{},
	}
	statusRows, err := r.db.QueryContext(ctx, `
SELECT name, site_status_ok, site_status_message, quota_display_type, quota_per_unit, custom_currency_symbol
FROM newapi_checkin_sites
ORDER BY id ASC`)
	if err != nil {
		return payload, err
	}
	for statusRows.Next() {
		var name string
		var status service.NewAPICheckinSiteStatus
		if err := statusRows.Scan(&name, &status.OK, &status.Message, &status.QuotaDisplayType, &status.QuotaPerUnit, &status.CustomCurrencySymbol); err != nil {
			_ = statusRows.Close()
			return payload, err
		}
		payload.SiteStatuses[name] = status
	}
	if err := statusRows.Close(); err != nil {
		return payload, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT s.name, s.enabled, a.name, a.username, a.display_name, a.user_id, a.ip_profile,
       COALESCE(b.status,''), COALESCE(b.message,''), b.checkin_ok, b.checkin_success, b.checked_in_today,
       COALESCE(b.checkin_status,''), COALESCE(b.checkin_status_tone,''), COALESCE(b.checkin_message,''), COALESCE(b.checkin_date,''),
       b.quota, COALESCE(b.quota_display,''), b.used_quota, COALESCE(b.used_quota_display,''),
       b.quota_awarded, COALESCE(b.quota_awarded_display,''), COALESCE(b.last_refreshed_at,'')
FROM newapi_checkin_accounts a
JOIN newapi_checkin_sites s ON s.id = a.site_id
LEFT JOIN newapi_checkin_account_balances b ON b.account_id = a.id
WHERE a.enabled = TRUE
ORDER BY s.id ASC, a.id ASC`)
	if err != nil {
		return payload, err
	}
	defer rows.Close()
	for rows.Next() {
		var row service.NewAPICheckinBalanceAccount
		var checkinOK, checkinSuccess, checkedInToday sql.NullBool
		var quota, usedQuota, quotaAwarded sql.NullInt64
		if err := rows.Scan(
			&row.Site, &row.Enabled, &row.Account, &row.Username, &row.DisplayName, &row.UserID, &row.IPProfile,
			&row.Status, &row.Message, &checkinOK, &checkinSuccess, &checkedInToday,
			&row.CheckinStatus, &row.CheckinStatusTone, &row.CheckinMessage, &row.CheckinDate,
			&quota, &row.QuotaDisplay, &usedQuota, &row.UsedQuotaDisplay,
			&quotaAwarded, &row.QuotaAwardedDisplay, &row.LastRefreshedAt,
		); err != nil {
			return payload, err
		}
		row.CheckinOK = nullableBoolValue(checkinOK)
		row.CheckinSuccess = nullableBoolValue(checkinSuccess)
		row.CheckedInToday = nullableBoolValue(checkedInToday)
		row.Quota = int64PtrFromNull(quota)
		row.UsedQuota = int64PtrFromNull(usedQuota)
		row.QuotaAwarded = int64PtrFromNull(quotaAwarded)
		row.Label = firstNonEmptyRepository(row.DisplayName, row.Username, row.Account, row.UserID)
		if row.LastRefreshedAt > payload.GeneratedAt {
			payload.GeneratedAt = row.LastRefreshedAt
		}
		payload.Accounts = append(payload.Accounts, row)
	}
	return payload, rows.Err()
}

func (r *newAPICheckinRepository) SaveBalanceCache(ctx context.Context, cache service.NewAPICheckinBalancePayload) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for siteName, status := range cache.SiteStatuses {
		status = normalizeRepositorySiteStatus(status)
		if _, err := tx.ExecContext(ctx, `
UPDATE newapi_checkin_sites
SET site_status_ok = $2,
    site_status_message = $3,
    quota_display_type = $4,
    quota_per_unit = $5,
    custom_currency_symbol = $6,
    updated_at = NOW()
WHERE name = $1`,
			siteName, status.OK, status.Message, status.QuotaDisplayType, status.QuotaPerUnit, status.CustomCurrencySymbol,
		); err != nil {
			return err
		}
	}

	for _, row := range cache.Accounts {
		accountID, err := lookupNewAPICheckinAccountID(ctx, tx, row.Site, row.UserID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_account_balances (
  account_id, status, message, checkin_ok, checkin_success, checked_in_today, checkin_status, checkin_status_tone,
  checkin_message, checkin_date, quota, quota_display, used_quota, used_quota_display,
  quota_awarded, quota_awarded_display, last_refreshed_at, source, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())
ON CONFLICT (account_id) DO UPDATE SET
  status = EXCLUDED.status,
  message = EXCLUDED.message,
  checkin_ok = EXCLUDED.checkin_ok,
  checkin_success = EXCLUDED.checkin_success,
  checked_in_today = EXCLUDED.checked_in_today,
  checkin_status = EXCLUDED.checkin_status,
  checkin_status_tone = EXCLUDED.checkin_status_tone,
  checkin_message = EXCLUDED.checkin_message,
  checkin_date = EXCLUDED.checkin_date,
  quota = EXCLUDED.quota,
  quota_display = EXCLUDED.quota_display,
  used_quota = EXCLUDED.used_quota,
  used_quota_display = EXCLUDED.used_quota_display,
  quota_awarded = EXCLUDED.quota_awarded,
  quota_awarded_display = EXCLUDED.quota_awarded_display,
  last_refreshed_at = EXCLUDED.last_refreshed_at,
  source = EXCLUDED.source,
  updated_at = NOW()`,
			accountID, row.Status, row.Message, nullableBoolAny(row.CheckinOK), nullableBoolAny(row.CheckinSuccess), nullableBoolAny(row.CheckedInToday),
			row.CheckinStatus, row.CheckinStatusTone, row.CheckinMessage, row.CheckinDate, nullableInt64Ptr(row.Quota), row.QuotaDisplay,
			nullableInt64Ptr(row.UsedQuota), row.UsedQuotaDisplay, nullableInt64Ptr(row.QuotaAwarded), row.QuotaAwardedDisplay,
			row.LastRefreshedAt, cache.Source,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *newAPICheckinRepository) LoadHistory(ctx context.Context) (service.NewAPICheckinHistoryPayload, error) {
	var payload service.NewAPICheckinHistoryPayload
	rows, err := r.db.QueryContext(ctx, `
SELECT date, recorded_at, source, site, account, user_id, ip_profile, checkin_status, checkin_status_tone, checkin_message,
       checkin_success, checked_in_today, quota_awarded, quota_awarded_display, quota_awarded_display_value,
       balance, balance_display, balance_display_value, used_quota, used_display, used_display_value
FROM newapi_checkin_history
ORDER BY date ASC, site ASC, user_id ASC`)
	if err != nil {
		return payload, err
	}
	defer rows.Close()
	for rows.Next() {
		entry, err := scanNewAPICheckinHistoryEntry(rows)
		if err != nil {
			return payload, err
		}
		payload.Entries = append(payload.Entries, entry)
	}
	return payload, rows.Err()
}

func (r *newAPICheckinRepository) SaveHistory(ctx context.Context, payload service.NewAPICheckinHistoryPayload) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM newapi_checkin_history`); err != nil {
		return err
	}
	for _, entry := range payload.Entries {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_history (
  date, recorded_at, source, site, account, user_id, ip_profile, checkin_status, checkin_status_tone, checkin_message,
  checkin_success, checked_in_today, quota_awarded, quota_awarded_display, quota_awarded_display_value,
  balance, balance_display, balance_display_value, used_quota, used_display, used_display_value
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
ON CONFLICT (site, user_id, date) DO UPDATE SET
  recorded_at = EXCLUDED.recorded_at,
  source = EXCLUDED.source,
  account = EXCLUDED.account,
  ip_profile = EXCLUDED.ip_profile,
  checkin_status = EXCLUDED.checkin_status,
  checkin_status_tone = EXCLUDED.checkin_status_tone,
  checkin_message = EXCLUDED.checkin_message,
  checkin_success = EXCLUDED.checkin_success,
  checked_in_today = EXCLUDED.checked_in_today,
  quota_awarded = EXCLUDED.quota_awarded,
  quota_awarded_display = EXCLUDED.quota_awarded_display,
  quota_awarded_display_value = EXCLUDED.quota_awarded_display_value,
  balance = EXCLUDED.balance,
  balance_display = EXCLUDED.balance_display,
  balance_display_value = EXCLUDED.balance_display_value,
  used_quota = EXCLUDED.used_quota,
  used_display = EXCLUDED.used_display,
  used_display_value = EXCLUDED.used_display_value`,
			entry.Date, entry.RecordedAt, entry.Source, entry.Site, entry.Account, entry.UserID, entry.IPProfile,
			entry.CheckinStatus, entry.CheckinStatusTone, entry.CheckinMessage, entry.CheckinSuccess, entry.CheckedInToday,
			nullableInt64Ptr(entry.QuotaAwarded), entry.QuotaAwardedDisplay, entry.QuotaAwardedDisplayValue,
			nullableInt64Ptr(entry.Balance), entry.BalanceDisplay, entry.BalanceDisplayValue,
			nullableInt64Ptr(entry.UsedQuota), entry.UsedDisplay, entry.UsedDisplayValue,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *newAPICheckinRepository) LoadMonthlyRecords(ctx context.Context) ([]service.NewAPICheckinMonthlyRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT site, user_id, account_name, username, display_name, ip_profile, month, checkin_date,
       quota_awarded, quota_awarded_display, quota_awarded_display_value, fetched_at, source
FROM newapi_checkin_monthly_records
ORDER BY checkin_date ASC, site ASC, user_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []service.NewAPICheckinMonthlyRecord{}
	for rows.Next() {
		record, err := scanNewAPICheckinMonthlyRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *newAPICheckinRepository) SaveMonthlyRecords(ctx context.Context, records []service.NewAPICheckinMonthlyRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM newapi_checkin_monthly_records`); err != nil {
		return err
	}
	for _, record := range records {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO newapi_checkin_monthly_records (
  site, user_id, account_name, username, display_name, ip_profile, month, checkin_date,
  quota_awarded, quota_awarded_display, quota_awarded_display_value, fetched_at, source
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (site, user_id, checkin_date) DO UPDATE SET
  account_name = EXCLUDED.account_name,
  username = EXCLUDED.username,
  display_name = EXCLUDED.display_name,
  ip_profile = EXCLUDED.ip_profile,
  month = EXCLUDED.month,
  quota_awarded = EXCLUDED.quota_awarded,
  quota_awarded_display = EXCLUDED.quota_awarded_display,
  quota_awarded_display_value = EXCLUDED.quota_awarded_display_value,
  fetched_at = EXCLUDED.fetched_at,
  source = EXCLUDED.source`,
			record.Site, record.UserID, record.AccountName, record.Username, record.DisplayName, record.IPProfile,
			record.Month, record.CheckinDate, nullableInt64Ptr(record.QuotaAwarded), record.QuotaAwardedDisplay,
			record.QuotaAwardedDisplayValue, record.FetchedAt, record.Source,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanNewAPICheckinRunResult(scanner interface{ Scan(...any) error }) (service.NewAPICheckinAccountResult, error) {
	var row service.NewAPICheckinAccountResult
	var quotaAwarded, remainingQuota, usedQuota sql.NullInt64
	if err := scanner.Scan(
		&row.Site, &row.Account, &row.UserID, &row.IPProfile, &row.OK, &row.Success, &row.Message, &row.CheckinDate,
		&quotaAwarded, &row.QuotaAwardedDisplay, &row.QuotaAwardedDisplayValue,
		&remainingQuota, &row.RemainingQuotaDisplay, &usedQuota, &row.UsedQuotaDisplay,
		&row.Username, &row.DisplayName, &row.Status, &row.CheckinStatus, &row.CheckinStatusTone,
	); err != nil {
		return row, err
	}
	row.QuotaAwarded = int64PtrFromNull(quotaAwarded)
	row.RemainingQuota = int64PtrFromNull(remainingQuota)
	row.UsedQuota = int64PtrFromNull(usedQuota)
	return row, nil
}

func scanNewAPICheckinHistoryEntry(scanner interface{ Scan(...any) error }) (service.NewAPICheckinHistoryEntry, error) {
	var entry service.NewAPICheckinHistoryEntry
	var quotaAwarded, balance, usedQuota sql.NullInt64
	if err := scanner.Scan(
		&entry.Date, &entry.RecordedAt, &entry.Source, &entry.Site, &entry.Account, &entry.UserID, &entry.IPProfile,
		&entry.CheckinStatus, &entry.CheckinStatusTone, &entry.CheckinMessage, &entry.CheckinSuccess, &entry.CheckedInToday,
		&quotaAwarded, &entry.QuotaAwardedDisplay, &entry.QuotaAwardedDisplayValue,
		&balance, &entry.BalanceDisplay, &entry.BalanceDisplayValue,
		&usedQuota, &entry.UsedDisplay, &entry.UsedDisplayValue,
	); err != nil {
		return entry, err
	}
	entry.QuotaAwarded = int64PtrFromNull(quotaAwarded)
	entry.Balance = int64PtrFromNull(balance)
	entry.UsedQuota = int64PtrFromNull(usedQuota)
	return entry, nil
}

func scanNewAPICheckinMonthlyRecord(scanner interface{ Scan(...any) error }) (service.NewAPICheckinMonthlyRecord, error) {
	var record service.NewAPICheckinMonthlyRecord
	var quotaAwarded sql.NullInt64
	if err := scanner.Scan(
		&record.Site, &record.UserID, &record.AccountName, &record.Username, &record.DisplayName, &record.IPProfile,
		&record.Month, &record.CheckinDate, &quotaAwarded, &record.QuotaAwardedDisplay, &record.QuotaAwardedDisplayValue,
		&record.FetchedAt, &record.Source,
	); err != nil {
		return record, err
	}
	record.QuotaAwarded = int64PtrFromNull(quotaAwarded)
	return record, nil
}

func lookupNewAPICheckinAccountID(ctx context.Context, tx *sql.Tx, siteName, userID string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
SELECT a.id
FROM newapi_checkin_accounts a
JOIN newapi_checkin_sites s ON s.id = a.site_id
WHERE s.name = $1 AND a.user_id = $2`, siteName, userID).Scan(&id)
	return id, err
}

func normalizeRepositorySiteStatus(status service.NewAPICheckinSiteStatus) service.NewAPICheckinSiteStatus {
	if status.QuotaDisplayType == "" {
		status.QuotaDisplayType = "USD"
	}
	if status.QuotaPerUnit <= 0 {
		status.QuotaPerUnit = 500000
	}
	return status
}

func nullableInt64Ptr(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func nullableBoolValue(value sql.NullBool) any {
	if !value.Valid {
		return nil
	}
	return value.Bool
}

func nullableBoolAny(value any) any {
	if v, ok := value.(bool); ok {
		return v
	}
	return nil
}

func firstNonEmptyRepository(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var _ service.NewAPICheckinRepository = (*newAPICheckinRepository)(nil)

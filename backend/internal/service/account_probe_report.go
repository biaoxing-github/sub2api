package service

import (
	"sort"
	"strings"
	"time"
)

const (
	AccountProbeReportSortScore       = "score"
	AccountProbeReportSortCreatedAt   = "created_at"
	AccountProbeReportSortSuccessRate = "success_rate"
	AccountProbeReportSortAvgLatency  = "avg_latency_ms"
	AccountProbeReportSortP95Latency  = "p95_ms"
	AccountProbeReportSortFirstToken  = "first_token_ms"
	AccountProbeReportSortTotalTokens = "total_tokens"
)

func normalizeAccountProbeReportFilter(filter AccountProbeReportFilter) AccountProbeReportFilter {
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Profile = strings.TrimSpace(filter.Profile)
	filter.RequestMode = strings.TrimSpace(filter.RequestMode)
	filter.Model = strings.TrimSpace(filter.Model)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	filter.Sort = normalizeAccountProbeReportSort(filter.Sort)
	filter.Order = normalizeAccountProbeReportOrder(filter.Order)
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return filter
}

func normalizeAccountProbeReportSort(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case AccountProbeReportSortScore:
		return AccountProbeReportSortScore
	case AccountProbeReportSortSuccessRate:
		return AccountProbeReportSortSuccessRate
	case AccountProbeReportSortAvgLatency:
		return AccountProbeReportSortAvgLatency
	case AccountProbeReportSortP95Latency:
		return AccountProbeReportSortP95Latency
	case AccountProbeReportSortFirstToken:
		return AccountProbeReportSortFirstToken
	case AccountProbeReportSortTotalTokens:
		return AccountProbeReportSortTotalTokens
	case AccountProbeReportSortCreatedAt, "":
		return AccountProbeReportSortCreatedAt
	default:
		return AccountProbeReportSortCreatedAt
	}
}

func normalizeAccountProbeReportOrder(order string) string {
	if strings.EqualFold(strings.TrimSpace(order), "asc") {
		return "asc"
	}
	return "desc"
}

func decorateAccountProbeReportItem(item *AccountProbeReportItem) {
	if item == nil {
		return
	}
	score := ScoreAccountProbeRun(item.AccountProbeResult)
	item.Score = score.Score
	item.Grade = score.Grade
	item.GradeLabel = score.Label
	item.Confidence = score.Confidence
	item.ScoreItems = score.ScoreItems
	item.PenaltyItems = score.PenaltyItems
	item.SuccessRate = accountProbeResultSuccessRate(item.AccountProbeResult)
}

func accountProbeResultSuccessRate(run AccountProbeResult) float64 {
	total := run.RequestCount
	if total <= 0 {
		total = run.SuccessCount + run.FailureCount
	}
	if total <= 0 {
		return 0
	}
	return float64(run.SuccessCount) / float64(total)
}

func buildAccountProbeReportSummary(items []AccountProbeReportItem, total int) AccountProbeReportSummary {
	var sum, scored int
	summary := AccountProbeReportSummary{Total: total}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, item := range items {
		if item.Score > 0 {
			sum += item.Score
			scored++
		}
		switch item.Grade {
		case AccountProbeGradeExcellent:
			summary.ExcellentCount++
		case AccountProbeGradeUnstable, AccountProbeGradePoor:
			summary.UnstableCount++
		}
		if item.Status == AccountProbeStatusRunning {
			summary.RunningCount++
		}
		if !item.CreatedAt.IsZero() && item.CreatedAt.After(cutoff) {
			summary.Recent24HourCount++
		}
	}
	if scored > 0 {
		summary.AverageScore = float64(sum) / float64(scored)
	}
	return summary
}

func buildAccountProbeRankingItems(runs []AccountProbeReportItem) []AccountProbeRankingItem {
	grouped := make(map[int64][]AccountProbeReportItem)
	for _, run := range runs {
		if run.AccountID <= 0 {
			continue
		}
		decorateAccountProbeReportItem(&run)
		grouped[run.AccountID] = append(grouped[run.AccountID], run)
	}

	items := make([]AccountProbeRankingItem, 0, len(grouped))
	for accountID, accountRuns := range grouped {
		sort.SliceStable(accountRuns, func(i, j int) bool {
			return accountRuns[i].CreatedAt.After(accountRuns[j].CreatedAt)
		})
		latest := accountRuns[0]
		item := AccountProbeRankingItem{
			AccountID:       accountID,
			AccountName:     latest.AccountName,
			RunCount:        len(accountRuns),
			LatestScore:     latest.Score,
			Grade:           latest.Grade,
			GradeLabel:      latest.GradeLabel,
			LatestRunID:     latest.ID,
			LatestStatus:    latest.Status,
			LatestCreatedAt: latest.CreatedAt,
			LatestModel:     latest.Model,
			ScoreHistory:    make([]AccountProbeScorePoint, 0, len(accountRuns)),
		}
		var scoreSum, successRateSum, latencySum float64
		var scored, successRated, latencyCount int
		for i := len(accountRuns) - 1; i >= 0; i-- {
			run := accountRuns[i]
			if run.Score > 0 {
				scoreSum += float64(run.Score)
				scored++
			}
			if run.RequestCount > 0 || run.SuccessCount+run.FailureCount > 0 {
				successRateSum += run.SuccessRate
				successRated++
			}
			avgLatency := run.Latency.AvgMillis
			if avgLatency == 0 {
				avgLatency = run.AvgLatencyMillis
			}
			if avgLatency > 0 {
				latencySum += float64(avgLatency)
				latencyCount++
			}
			item.ScoreHistory = append(item.ScoreHistory, AccountProbeScorePoint{
				RunID:            run.ID,
				Score:            run.Score,
				Grade:            run.Grade,
				GradeLabel:       run.GradeLabel,
				Status:           run.Status,
				Model:            run.Model,
				Profile:          run.Profile,
				RequestMode:      run.RequestMode,
				SuccessRate:      run.SuccessRate,
				AvgLatencyMillis: avgLatency,
				P95LatencyMillis: run.Latency.P95Millis,
				FirstTokenMillis: run.FirstTokenMillis,
				TotalTokens:      run.TotalTokens,
				CreatedAt:        run.CreatedAt,
			})
		}
		if scored > 0 {
			item.AverageScore = scoreSum / float64(scored)
		}
		if successRated > 0 {
			item.AverageSuccessRate = successRateSum / float64(successRated)
		}
		if latencyCount > 0 {
			item.AverageLatencyMillis = latencySum / float64(latencyCount)
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].AverageScore != items[j].AverageScore {
			return items[i].AverageScore > items[j].AverageScore
		}
		if items[i].LatestScore != items[j].LatestScore {
			return items[i].LatestScore > items[j].LatestScore
		}
		return items[i].LatestCreatedAt.After(items[j].LatestCreatedAt)
	})
	return items
}

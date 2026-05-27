package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	OpenAIContinuityActionSticky     = "sticky"
	OpenAIContinuityActionReplay     = "replay"
	OpenAIContinuityActionProtected  = "protected"
	OpenAIContinuityActionNewSession = "new-session"

	OpenAIContinuityReasonBalanceOK        = "balance_ok"
	OpenAIContinuityReasonBalanceUnknown   = "balance_unknown"
	OpenAIContinuityReasonBalanceExhausted = "balance_exhausted"
	OpenAIContinuityReasonReplayNotSafe    = "replay_not_safe"
	OpenAIContinuityReasonJournalMissing   = "journal_missing"
)

type OpenAIContextContinuityError struct {
	Code               string
	Message            string
	SessionHash        string
	PreviousResponseID string
	CurrentAccountID   int64
	Reason             string
	Detail             map[string]any
}

func (e *OpenAIContextContinuityError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Code) != "" {
		return e.Code
	}
	return "openai context continuity error"
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerAndContinuity(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
	requestBody []byte,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	migration := ClassifyOpenAIContextMigration(requestBody, false)
	selection, decision, err := s.SelectAccountWithScheduler(
		ctx,
		groupID,
		previousResponseID,
		sessionHash,
		requestedModel,
		excludedIDs,
		requiredTransport,
		requireCompact,
	)
	applyOpenAIContextMigrationToDecision(&decision, migration)
	if err != nil || selection == nil || selection.Account == nil {
		return selection, decision, err
	}

	previousResponseID = strings.TrimSpace(previousResponseID)
	if decision.Layer != openAIAccountScheduleLayerPreviousResponse &&
		decision.Layer != openAIAccountScheduleLayerSessionSticky {
		if previousResponseID != "" {
			fromAccountID, ok := s.lookupOpenAIContinuitySourceAccountID(ctx, groupID, previousResponseID, sessionHash)
			if ok && fromAccountID > 0 && fromAccountID != selection.Account.ID {
				replayBody, replayReason, replayDetail, replayOK := s.buildOpenAIContinuityReplayBody(ctx, groupID, previousResponseID, sessionHash, requestBody)
				if !replayOK {
					if replayReason == "" {
						replayReason = OpenAIContinuityReasonReplayNotSafe
					}
					releaseOpenAISelection(selection)
					decision.ContinuityAction = OpenAIContinuityActionProtected
					decision.ContinuityReason = replayReason
					decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), replayDetail)
					decision.ContinuityFromAccountID = fromAccountID
					return nil, decision, &OpenAIContextContinuityError{
						Code:               "context_replay_not_safe",
						Message:            "Current session depends on upstream state that cannot be safely replayed to another account.",
						SessionHash:        sessionHash,
						PreviousResponseID: previousResponseID,
						CurrentAccountID:   fromAccountID,
						Reason:             replayReason,
						Detail:             decision.ContinuityDetail,
					}
				}
				decision.ContinuityAction = OpenAIContinuityActionReplay
				decision.ContinuityReason = OpenAIContinuityReasonBalanceExhausted
				decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), replayDetail)
				decision.ContinuityFromAccountID = fromAccountID
				decision.ContinuityReplayBody = cloneBytes(replayBody)
				return selection, decision, nil
			}
		}
		decision.ContinuityAction = OpenAIContinuityActionNewSession
		decision.ContinuityDetail = mergeContinuityDetails(decision.ContinuityDetail, migration.DetailMap())
		return selection, decision, nil
	}

	account := selection.Account
	available, checked, balanceDetail, checkErr := s.checkRealtimeBalanceAvailableForContinuity(ctx, account)
	if !checked {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceOK
		decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail)
		return selection, decision, nil
	}
	if checkErr != nil {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceUnknown
		decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail)
		return selection, decision, nil
	}
	if available {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceOK
		decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail)
		return selection, decision, nil
	}

	releaseOpenAISelection(selection)
	replayBody, replayReason, replayDetail, replayOK := s.buildOpenAIContinuityReplayBody(ctx, groupID, previousResponseID, sessionHash, requestBody)
	if !replayOK {
		if replayReason == "" {
			replayReason = OpenAIContinuityReasonReplayNotSafe
		}
		decision.ContinuityAction = OpenAIContinuityActionProtected
		decision.ContinuityReason = replayReason
		decision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail, replayDetail)
		decision.ContinuityFromAccountID = account.ID
		return nil, decision, &OpenAIContextContinuityError{
			Code:               "context_replay_not_safe",
			Message:            "Current session depends on upstream state that cannot be safely replayed to another account.",
			SessionHash:        sessionHash,
			PreviousResponseID: strings.TrimSpace(previousResponseID),
			CurrentAccountID:   account.ID,
			Reason:             replayReason,
			Detail:             decision.ContinuityDetail,
		}
	}

	replayExcludedIDs := cloneExcludedAccountIDs(excludedIDs)
	if replayExcludedIDs == nil {
		replayExcludedIDs = make(map[int64]struct{}, 1)
	}
	replayExcludedIDs[account.ID] = struct{}{}
	nextSelection, nextDecision, selectErr := s.SelectAccountWithScheduler(
		ctx,
		groupID,
		"",
		sessionHash,
		requestedModel,
		replayExcludedIDs,
		requiredTransport,
		requireCompact,
	)
	if selectErr != nil || nextSelection == nil || nextSelection.Account == nil {
		applyOpenAIContextMigrationToDecision(&nextDecision, migration)
		nextDecision.ContinuityAction = OpenAIContinuityActionProtected
		nextDecision.ContinuityReason = OpenAIContinuityReasonBalanceExhausted
		nextDecision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail)
		nextDecision.ContinuityFromAccountID = account.ID
		return nextSelection, nextDecision, selectErr
	}
	applyOpenAIContextMigrationToDecision(&nextDecision, migration)
	if nextDecision.Layer == openAIAccountScheduleLayerLoadBalance &&
		strings.TrimSpace(previousResponseID) != "" &&
		nextSelection.Account.ID != account.ID {
		nextDecision.ContinuityReplayBody = cloneBytes(replayBody)
	}
	nextDecision.ContinuityAction = OpenAIContinuityActionReplay
	nextDecision.ContinuityReason = OpenAIContinuityReasonBalanceExhausted
	nextDecision.ContinuityDetail = mergeContinuityDetails(migration.DetailMap(), balanceDetail, replayDetail)
	nextDecision.ContinuityFromAccountID = account.ID
	if len(nextDecision.ContinuityReplayBody) == 0 {
		nextDecision.ContinuityReplayBody = cloneBytes(replayBody)
	}
	return nextSelection, nextDecision, nil
}

func (s *OpenAIGatewayService) lookupOpenAIContinuitySourceAccountID(ctx context.Context, groupID *int64, previousResponseID string, sessionHash string) (int64, bool) {
	if s == nil {
		return 0, false
	}
	responseID := strings.TrimSpace(previousResponseID)
	if responseID == "" {
		return 0, false
	}
	resolvedGroupID := derefGroupID(groupID)
	if s.contextJournal != nil {
		ref, err := s.contextJournal.GetResponse(ctx, resolvedGroupID, responseID)
		if err == nil && ref != nil && ref.AccountID > 0 {
			if sessionHash == "" || strings.TrimSpace(ref.SessionHash) == "" || ref.SessionHash == sessionHash {
				return ref.AccountID, true
			}
		}
	}
	if store := s.getOpenAIWSStateStore(); store != nil {
		accountID, err := store.GetResponseAccount(ctx, resolvedGroupID, responseID)
		if err == nil && accountID > 0 {
			return accountID, true
		}
	}
	return 0, false
}

func (s *OpenAIGatewayService) checkRealtimeBalanceAvailableForContinuity(ctx context.Context, account *Account) (available bool, checked bool, detail map[string]any, err error) {
	if s == nil || s.realtimeBalanceChecker == nil || account == nil {
		return true, false, nil, nil
	}
	if account.Type != AccountTypeAPIKey || !account.IsOpenAI() {
		return true, false, nil, nil
	}
	decision, err := s.realtimeBalanceChecker.CheckAccountSnapshotFirst(ctx, account, RealtimeBalanceCheckOptions{SnapshotOnly: true})
	detail = continuityBalanceDetail(decision, err)
	if err != nil {
		return false, true, detail, err
	}
	if decision != nil && decision.State == RealtimeBalanceStateUnknown {
		reason := strings.TrimSpace(decision.Error)
		if reason == "" {
			reason = OpenAIContinuityReasonBalanceUnknown
		}
		return false, true, detail, errors.New(reason)
	}
	return decision != nil && decision.Available > 0, true, detail, nil
}

func (s *OpenAIGatewayService) buildOpenAIContinuityReplayBody(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestBody []byte,
) ([]byte, string, map[string]any, bool) {
	migration := ClassifyOpenAIContextMigration(requestBody, false)
	if !migration.IsPortable() && ClassifyContextReplaySafety(requestBody) != ContextReplaySafe {
		reason := OpenAIContinuityReasonReplayNotSafe
		return nil, reason, mergeContinuityDetails(migration.DetailMap(), continuityReplayDetail(false, true, reason)), false
	}

	responseID := strings.TrimSpace(previousResponseID)
	if responseID != "" && s != nil && s.contextJournal != nil {
		ref, err := s.contextJournal.GetResponse(ctx, derefGroupID(groupID), responseID)
		if err != nil || ref == nil {
			reason := OpenAIContinuityReasonJournalMissing
			return nil, reason, continuityReplayDetail(false, true, reason), false
		}
		if sessionHash != "" && strings.TrimSpace(ref.SessionHash) != "" && ref.SessionHash != sessionHash {
			reason := OpenAIContinuityReasonJournalMissing
			return nil, reason, continuityReplayDetail(false, true, reason), false
		}
		migration = markOpenAIContextMigrationSnapshotReplayable(migration, "context_journal_response_snapshot_replayable")
	}

	replayBody := cloneBytes(requestBody)
	if responseID != "" {
		updated, _, err := dropPreviousResponseIDFromRawPayload(replayBody)
		if err != nil {
			reason := fmt.Sprintf("%s:%s", OpenAIContinuityReasonReplayNotSafe, err.Error())
			return nil, reason, continuityReplayDetail(false, true, reason), false
		}
		replayBody = updated
	}
	replayMigration := ClassifyOpenAIContextMigration(replayBody, false)
	if !replayMigration.IsPortable() && ClassifyContextReplaySafety(replayBody) != ContextReplaySafe {
		reason := OpenAIContinuityReasonReplayNotSafe
		return nil, reason, mergeContinuityDetails(replayMigration.DetailMap(), continuityReplayDetail(false, true, reason)), false
	}
	return replayBody, "", mergeContinuityDetails(replayMigration.DetailMap(), continuityReplayDetail(true, false, "replay_safe")), true
}

func applyOpenAIContextMigrationToDecision(decision *OpenAIAccountScheduleDecision, migration OpenAIContextMigrationObservation) {
	if decision == nil {
		return
	}
	decision.ContextMigrationClass = migration.Class
	decision.ContextMigrationReason = migration.Reason
	decision.ContextMigrationDetail = migration.DetailMap()
	if len(decision.ContinuityDetail) == 0 {
		decision.ContinuityDetail = migration.DetailMap()
	}
}

func continuityBalanceDetail(decision *RealtimeBalanceDecision, err error) map[string]any {
	detail := map[string]any{
		"replay_safe": false,
	}
	if decision != nil {
		detail["balance_confirm_source"] = decision.Source
		detail["available"] = decision.Available
		detail["state"] = decision.State
		detail["threshold"] = decision.Threshold
		if !decision.CheckedAt.IsZero() {
			detail["checked_at"] = decision.CheckedAt
		}
		if reason := strings.TrimSpace(decision.Error); reason != "" {
			detail["reason"] = reason
		}
	}
	if err != nil {
		detail["balance_confirm_source"] = RealtimeBalanceSourceError
		detail["reason"] = err.Error()
	}
	if _, ok := detail["reason"]; !ok {
		if decision != nil && decision.Available > 0 {
			detail["reason"] = OpenAIContinuityReasonBalanceOK
		} else {
			detail["reason"] = OpenAIContinuityReasonBalanceExhausted
		}
	}
	return detail
}

func continuityReplayDetail(replaySafe bool, protected bool, reason string) map[string]any {
	detail := map[string]any{
		"replay_safe": replaySafe,
		"protected":   protected,
	}
	if reason = strings.TrimSpace(reason); reason != "" {
		detail["reason"] = reason
	}
	return detail
}

func mergeContinuityDetails(details ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, detail := range details {
		for k, v := range detail {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func releaseOpenAISelection(selection *AccountSelectionResult) {
	if selection == nil || selection.ReleaseFunc == nil {
		return
	}
	selection.ReleaseFunc()
	selection.ReleaseFunc = nil
	selection.Acquired = false
}

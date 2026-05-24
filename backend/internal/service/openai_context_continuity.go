package service

import (
	"context"
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
	if err != nil || selection == nil || selection.Account == nil {
		return selection, decision, err
	}

	if decision.Layer != openAIAccountScheduleLayerPreviousResponse &&
		decision.Layer != openAIAccountScheduleLayerSessionSticky {
		decision.ContinuityAction = OpenAIContinuityActionNewSession
		return selection, decision, nil
	}

	account := selection.Account
	available, checked, checkErr := s.checkRealtimeBalanceAvailableForContinuity(ctx, account)
	if !checked {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceOK
		return selection, decision, nil
	}
	if checkErr != nil {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceUnknown
		return selection, decision, nil
	}
	if available {
		decision.ContinuityAction = OpenAIContinuityActionSticky
		decision.ContinuityReason = OpenAIContinuityReasonBalanceOK
		return selection, decision, nil
	}

	releaseOpenAISelection(selection)
	replayBody, replayReason, replayOK := s.buildOpenAIContinuityReplayBody(ctx, groupID, previousResponseID, sessionHash, requestBody)
	if !replayOK {
		if replayReason == "" {
			replayReason = OpenAIContinuityReasonReplayNotSafe
		}
		decision.ContinuityAction = OpenAIContinuityActionProtected
		decision.ContinuityReason = replayReason
		decision.ContinuityFromAccountID = account.ID
		return nil, decision, &OpenAIContextContinuityError{
			Code:               "context_replay_not_safe",
			Message:            "Current session depends on upstream state that cannot be safely replayed to another account.",
			SessionHash:        sessionHash,
			PreviousResponseID: strings.TrimSpace(previousResponseID),
			CurrentAccountID:   account.ID,
			Reason:             replayReason,
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
		nextDecision.ContinuityAction = OpenAIContinuityActionProtected
		nextDecision.ContinuityReason = OpenAIContinuityReasonBalanceExhausted
		nextDecision.ContinuityFromAccountID = account.ID
		return nextSelection, nextDecision, selectErr
	}
	nextDecision.ContinuityAction = OpenAIContinuityActionReplay
	nextDecision.ContinuityReason = OpenAIContinuityReasonBalanceExhausted
	nextDecision.ContinuityFromAccountID = account.ID
	nextDecision.ContinuityReplayBody = cloneBytes(replayBody)
	return nextSelection, nextDecision, nil
}

func (s *OpenAIGatewayService) checkRealtimeBalanceAvailableForContinuity(ctx context.Context, account *Account) (available bool, checked bool, err error) {
	if s == nil || s.realtimeBalanceChecker == nil || account == nil {
		return true, false, nil
	}
	if account.Type != AccountTypeAPIKey || !account.IsOpenAI() {
		return true, false, nil
	}
	snapshot, err := s.realtimeBalanceChecker.CheckAccount(ctx, account)
	if err != nil {
		return false, true, err
	}
	return snapshot != nil && snapshot.Available > 0, true, nil
}

func (s *OpenAIGatewayService) buildOpenAIContinuityReplayBody(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestBody []byte,
) ([]byte, string, bool) {
	if ClassifyContextReplaySafety(requestBody) != ContextReplaySafe {
		return nil, OpenAIContinuityReasonReplayNotSafe, false
	}

	responseID := strings.TrimSpace(previousResponseID)
	if responseID != "" && s != nil && s.contextJournal != nil {
		ref, err := s.contextJournal.GetResponse(ctx, derefGroupID(groupID), responseID)
		if err != nil || ref == nil {
			return nil, OpenAIContinuityReasonJournalMissing, false
		}
		if sessionHash != "" && strings.TrimSpace(ref.SessionHash) != "" && ref.SessionHash != sessionHash {
			return nil, OpenAIContinuityReasonJournalMissing, false
		}
	}

	replayBody := cloneBytes(requestBody)
	if responseID != "" {
		updated, _, err := dropPreviousResponseIDFromRawPayload(replayBody)
		if err != nil {
			return nil, fmt.Sprintf("%s:%s", OpenAIContinuityReasonReplayNotSafe, err.Error()), false
		}
		replayBody = updated
	}
	if ClassifyContextReplaySafety(replayBody) != ContextReplaySafe {
		return nil, OpenAIContinuityReasonReplayNotSafe, false
	}
	return replayBody, "", true
}

func releaseOpenAISelection(selection *AccountSelectionResult) {
	if selection == nil || selection.ReleaseFunc == nil {
		return
	}
	selection.ReleaseFunc()
	selection.ReleaseFunc = nil
	selection.Acquired = false
}

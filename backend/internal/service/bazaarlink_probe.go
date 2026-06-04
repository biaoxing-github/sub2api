package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const (
	bazaarLinkProbeEndpoint       = "https://bazaarlink.ai/api/probe/run"
	bazaarLinkProbeRequestTimeout = 4 * time.Minute
	bazaarLinkProbePollInterval   = 1500 * time.Millisecond
)

type bazaarLinkProbePayload struct {
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey"`
	ModelID      string `json:"modelId"`
	ClaimedModel string `json:"claimedModel,omitempty"`
	QuickMode    bool   `json:"quickMode"`
	IdentityOnly bool   `json:"identityOnly"`
	Sync         bool   `json:"sync,omitempty"`
	Lang         string `json:"lang"`
}

type bazaarLinkProbeResponse struct {
	RunID              string                       `json:"runId"`
	Status             string                       `json:"status"`
	Score              int                          `json:"score"`
	IdentityAssessment bazaarLinkIdentityAssessment `json:"identityAssessment"`
	Items              []bazaarLinkProbeItem        `json:"items"`
	TotalInputTokens   *int                         `json:"totalInputTokens"`
	TotalOutputTokens  *int                         `json:"totalOutputTokens"`
}

type bazaarLinkIdentityAssessment struct {
	Status           string                     `json:"status"`
	Confidence       float64                    `json:"confidence"`
	ClaimedModel     string                     `json:"claimedModel"`
	PredictedFamily  string                     `json:"predictedFamily"`
	SubModelMatchV3F *bazaarLinkSubModelMatch   `json:"subModelMatchV3F"`
	RiskFlags        []string                   `json:"riskFlags"`
	V3               json.RawMessage            `json:"v3"`
	V3Candidates     []bazaarLinkModelCandidate `json:"v3Candidates"`
	Candidates       []bazaarLinkModelCandidate `json:"candidates"`
}

type bazaarLinkSubModelMatch struct {
	ModelID string  `json:"modelId"`
	Score   float64 `json:"score"`
}

type bazaarLinkCandidateGroup struct {
	Candidates    []bazaarLinkModelCandidate `json:"candidates"`
	TopCandidates []bazaarLinkModelCandidate `json:"topCandidates"`
	Matches       []bazaarLinkModelCandidate `json:"matches"`
}

type bazaarLinkModelCandidate struct {
	DisplayName string   `json:"displayName"`
	ModelID     string   `json:"modelId"`
	Family      string   `json:"family"`
	Score       *float64 `json:"score"`
}

type bazaarLinkProbeItem struct {
	ProbeID  string   `json:"probeId"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	Passed   any      `json:"passed"`
	Response string   `json:"response"`
	TTFTMs   *int     `json:"ttftMs"`
	TPS      *float64 `json:"tps"`
}

func normalizeBazaarLinkProbeMode(mode BazaarLinkProbeMode) (BazaarLinkProbeMode, error) {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(BazaarLinkProbeModeQuick):
		return BazaarLinkProbeModeQuick, nil
	case string(BazaarLinkProbeModeFull):
		return BazaarLinkProbeModeFull, nil
	default:
		return "", fmt.Errorf("unsupported BazaarLink probe mode: %s", strings.TrimSpace(string(mode)))
	}
}

func (s *AccountProbeService) StartBazaarLink(ctx context.Context, req BazaarLinkProbeRunRequest) (AccountProbeResult, error) {
	account, _, model, _, mode, err := s.prepareBazaarLinkProbe(ctx, req)
	if err != nil {
		return AccountProbeResult{}, err
	}

	now := time.Now()
	run := AccountProbeResult{
		AccountID:    account.ID,
		Profile:      AccountProbeProfileModelValidation,
		ProbeSource:  AccountProbeSourceBazaarLinkAPI,
		Status:       AccountProbeStatusRunning,
		Model:        model,
		RequestMode:  string(mode),
		Estimate:     AccountProbeEstimate{Requests: 1},
		RequestCount: 1,
		CreatedAt:    now,
		StartedAt:    &now,
	}
	if s.repo != nil {
		if err := s.repo.CreateAccountProbeRun(ctx, &run); err != nil {
			return AccountProbeResult{}, err
		}
	}
	return run, nil
}

func (s *AccountProbeService) RunBazaarLinkExisting(ctx context.Context, run AccountProbeResult, req BazaarLinkProbeRunRequest) (AccountProbeResult, error) {
	account, apiKey, model, baseURL, mode, err := s.prepareBazaarLinkProbe(ctx, req)
	if err != nil {
		return s.failExistingRun(ctx, run, err.Error()), err
	}
	run.AccountID = account.ID
	run.Profile = AccountProbeProfileModelValidation
	run.ProbeSource = AccountProbeSourceBazaarLinkAPI
	run.Model = model
	run.RequestMode = string(mode)
	run.RequestCount = 1
	run.Estimate = AccountProbeEstimate{Requests: 1}
	if run.Status == "" {
		run.Status = AccountProbeStatusRunning
	}
	if run.StartedAt == nil {
		now := time.Now()
		run.StartedAt = &now
	}

	sample := s.runBazaarLinkProbeSample(ctx, account, apiKey, model, baseURL, mode)
	sample.RunID = run.ID
	sample.RequestIndex = 1
	if s.repo != nil {
		persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
		err := s.repo.SaveAccountProbeSample(persistCtx, sample)
		cancel()
		if err != nil {
			return s.failExistingRun(ctx, run, err.Error()), err
		}
	}
	run.Samples = []AccountProbeSample{sample}
	finalizeAccountProbeResult(&run)
	if sample.Status == AccountProbeSampleFailed && sample.ErrorMessage != "" {
		run.Summary = sample.ErrorMessage
	}
	if s.repo != nil {
		persistCtx, cancel := context.WithTimeout(context.Background(), accountProbePersistenceTimeout)
		err := s.repo.UpdateAccountProbeRun(persistCtx, &run)
		cancel()
		if err != nil {
			return run, err
		}
	}
	return run, nil
}

func (s *AccountProbeService) prepareBazaarLinkProbe(ctx context.Context, req BazaarLinkProbeRunRequest) (*Account, string, string, string, BazaarLinkProbeMode, error) {
	if s == nil || s.accounts == nil {
		return nil, "", "", "", "", fmt.Errorf("account probe account repository is nil")
	}
	if req.AccountID <= 0 {
		return nil, "", "", "", "", fmt.Errorf("account_id is required")
	}
	mode, err := normalizeBazaarLinkProbeMode(req.Mode)
	if err != nil {
		return nil, "", "", "", "", err
	}
	account, err := s.accounts.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, "", "", "", "", err
	}
	if account == nil {
		return nil, "", "", "", "", ErrAccountNotFound
	}
	if !account.IsOpenAIApiKey() {
		return nil, "", "", "", "", fmt.Errorf("only openai api key accounts support BazaarLink probe runs")
	}
	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if apiKey == "" {
		return nil, "", "", "", "", fmt.Errorf("no api key available")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = openai.DefaultTestModel
	}
	model = account.GetMappedModel(model)
	baseURL := account.GetOpenAIPrimaryRequestBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com"
	}
	if s.testSvc != nil {
		normalized, err := s.testSvc.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, "", "", "", "", fmt.Errorf("invalid base URL: %w", err)
		}
		baseURL = normalized
	}
	baseURL = bazaarLinkOpenAIBaseURL(baseURL)
	return account, apiKey, model, baseURL, mode, nil
}

func (s *AccountProbeService) runBazaarLinkProbeSample(ctx context.Context, account *Account, apiKey, model, baseURL string, mode BazaarLinkProbeMode) AccountProbeSample {
	payload := bazaarLinkProbePayload{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		ModelID:      model,
		ClaimedModel: model,
		Lang:         "zh",
	}
	if mode == BazaarLinkProbeModeQuick {
		payload.QuickMode = true
		payload.IdentityOnly = true
	}
	requestBody := bazaarLinkProbeRequestBodyForDisplay(payload)
	data, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, bazaarLinkProbeRequestTimeout)
	defer cancel()
	start := time.Now()
	parsed, responseBody, httpStatus, err := s.runBazaarLinkProbeRequest(reqCtx, data)
	duration := time.Since(start)
	if err != nil {
		var probeErr *bazaarLinkProbeRequestError
		if errors.As(err, &probeErr) {
			return failedBazaarLinkProbeSample(account, model, mode, requestBody, probeErr.Code, bazaarLinkProbeErrorMessageForDisplay([]byte(probeErr.Body), probeErr.Message, apiKey), probeErr.HTTPStatus, duration)
		}
		return failedBazaarLinkProbeSample(account, model, mode, requestBody, "request_failed", err.Error(), 0, duration)
	}

	inputTokens := intFromOptional(parsed.TotalInputTokens)
	outputTokens := intFromOptional(parsed.TotalOutputTokens)
	evidence := bazaarLinkProbeEvidenceForMode(parsed, model, mode)
	status := AccountProbeSampleSuccess
	errorCode := ""
	errorMessage := ""
	if !evidence.Passed {
		status = AccountProbeSampleFailed
		errorCode = "bazaarlink_identity_mismatch"
		errorMessage = evidence.Message
	}
	return AccountProbeSample{
		Type:               AccountProbeSourceBazaarLinkAPI,
		Label:              bazaarLinkProbeLabel(mode),
		Status:             status,
		Model:              model,
		APIKeyFingerprint:  FingerprintAPIKey(apiKey),
		APIKeyMasked:       MaskAPIKey(apiKey),
		UpstreamEndpoint:   bazaarLinkProbeEndpoint,
		HTTPStatus:         httpStatus,
		DurationMillis:     int(math.Round(float64(duration / time.Millisecond))),
		InputTokens:        inputTokens,
		OutputTokens:       outputTokens,
		TotalTokens:        inputTokens + outputTokens,
		OutputText:         bazaarLinkProbeSummary(parsed),
		ValidationEvidence: []AccountProbeValidationEvidence{evidence},
		RequestBody:        requestBody,
		ResponseBody:       responseBody,
		ErrorCode:          errorCode,
		ErrorMessage:       errorMessage,
		CreatedAt:          time.Now(),
	}
}

type bazaarLinkProbeRequestError struct {
	Code       string
	Message    string
	Body       string
	HTTPStatus int
}

func (e *bazaarLinkProbeRequestError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (s *AccountProbeService) runBazaarLinkProbeRequest(ctx context.Context, data []byte) (bazaarLinkProbeResponse, string, int, error) {
	created, createBody, createStatus, err := s.sendBazaarLinkProbeJSON(ctx, http.MethodPost, bazaarLinkProbeEndpoint, data)
	if err != nil {
		return bazaarLinkProbeResponse{}, "", createStatus, err
	}
	if bazaarLinkProbeIsFinished(created.Status) {
		return created, createBody, createStatus, nil
	}
	runID := strings.TrimSpace(created.RunID)
	if runID == "" {
		return bazaarLinkProbeResponse{}, createBody, createStatus, &bazaarLinkProbeRequestError{
			Code:       "response_parse_failed",
			Message:    "BazaarLink async probe did not return runId",
			Body:       createBody,
			HTTPStatus: createStatus,
		}
	}

	pollURL := bazaarLinkProbeEndpoint + "/" + url.PathEscape(runID)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return bazaarLinkProbeResponse{}, createBody, createStatus, ctx.Err()
		case <-timer.C:
		}
		current, responseBody, httpStatus, err := s.sendBazaarLinkProbeJSON(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return bazaarLinkProbeResponse{}, responseBody, httpStatus, err
		}
		if bazaarLinkProbeIsFinished(current.Status) {
			if strings.TrimSpace(current.RunID) == "" {
				current.RunID = runID
			}
			return current, responseBody, httpStatus, nil
		}
		timer.Reset(bazaarLinkProbePollInterval)
	}
}

func (s *AccountProbeService) sendBazaarLinkProbeJSON(ctx context.Context, method, endpoint string, data []byte) (bazaarLinkProbeResponse, string, int, error) {
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return bazaarLinkProbeResponse{}, "", 0, &bazaarLinkProbeRequestError{Code: "request_create_failed", Message: err.Error()}
	}
	if data != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return bazaarLinkProbeResponse{}, "", 0, err
	}
	defer resp.Body.Close()
	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return bazaarLinkProbeResponse{}, "", resp.StatusCode, &bazaarLinkProbeRequestError{
			Code:       "response_read_failed",
			Message:    readErr.Error(),
			HTTPStatus: resp.StatusCode,
		}
	}
	responseBody := bazaarLinkProbeResponseBodyForDisplay(rawBody)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return bazaarLinkProbeResponse{}, responseBody, resp.StatusCode, &bazaarLinkProbeRequestError{
			Code:       fmt.Sprintf("http_%d", resp.StatusCode),
			Message:    resp.Status,
			Body:       responseBody,
			HTTPStatus: resp.StatusCode,
		}
	}
	var parsed bazaarLinkProbeResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return bazaarLinkProbeResponse{}, responseBody, resp.StatusCode, &bazaarLinkProbeRequestError{
			Code:       "response_parse_failed",
			Message:    err.Error(),
			Body:       responseBody,
			HTTPStatus: resp.StatusCode,
		}
	}
	return parsed, responseBody, resp.StatusCode, nil
}

func bazaarLinkProbeIsFinished(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "aborted", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func failedBazaarLinkProbeSample(account *Account, model string, mode BazaarLinkProbeMode, requestBody, code, message string, httpStatus int, duration time.Duration) AccountProbeSample {
	sample := AccountProbeSample{
		Type:             AccountProbeSourceBazaarLinkAPI,
		Label:            bazaarLinkProbeLabel(mode),
		Status:           AccountProbeSampleFailed,
		Model:            model,
		UpstreamEndpoint: bazaarLinkProbeEndpoint,
		HTTPStatus:       httpStatus,
		DurationMillis:   int(math.Round(float64(duration / time.Millisecond))),
		RequestBody:      requestBody,
		ErrorCode:        strings.TrimSpace(code),
		ErrorMessage:     strings.TrimSpace(message),
		CreatedAt:        time.Now(),
	}
	if account != nil {
		apiKey := account.GetOpenAIApiKey()
		sample.APIKeyFingerprint = FingerprintAPIKey(apiKey)
		sample.APIKeyMasked = MaskAPIKey(apiKey)
	}
	sample.ValidationEvidence = []AccountProbeValidationEvidence{{
		Key:      "bazaarlink_api",
		Label:    "BazaarLink API",
		Expected: "HTTP 2xx JSON result",
		Observed: strings.TrimSpace(message),
		Passed:   false,
		Score:    0,
		MaxScore: 100,
		Message:  strings.TrimSpace(message),
		Category: "external_api",
		Severity: "critical",
	}}
	return sample
}

func bazaarLinkProbeEvidence(result bazaarLinkProbeResponse, expectedModel string) AccountProbeValidationEvidence {
	return bazaarLinkProbeEvidenceForMode(result, expectedModel, BazaarLinkProbeModeFull)
}

func bazaarLinkProbeEvidenceForMode(result bazaarLinkProbeResponse, expectedModel string, mode BazaarLinkProbeMode) AccountProbeValidationEvidence {
	identity := result.IdentityAssessment
	statusConfirmed := bazaarLinkIdentityStatusConfirmed(identity.Status)
	hasRisk := len(identity.RiskFlags) > 0
	passed := statusConfirmed
	message := "BazaarLink 身份验证通过"
	severity := "info"
	if !passed {
		severity = "critical"
		message = "BazaarLink 未确认目标模型身份"
		if hasRisk {
			message += "：" + strings.Join(identity.RiskFlags, ", ")
		}
	} else if hasRisk {
		severity = "warning"
		message = "BazaarLink 身份验证通过，存在风险提示：" + strings.Join(identity.RiskFlags, ", ")
	}
	observed := bazaarLinkIdentityObserved(identity)
	score := bazaarLinkProbeEvidenceScore(result, expectedModel, mode)
	var displayScore *float64
	if mode == BazaarLinkProbeModeQuick {
		if candidateScore, ok := bazaarLinkClaimedCandidateScore(identity, expectedModel); ok {
			copiedScore := candidateScore
			displayScore = &copiedScore
		}
	}
	return AccountProbeValidationEvidence{
		Key:           "bazaarlink_identity",
		Label:         "BazaarLink 模型身份",
		Expected:      expectedModel,
		Observed:      observed,
		Passed:        passed,
		Score:         score,
		DisplayScore:  displayScore,
		MaxScore:      100,
		Message:       message,
		Category:      "external_api",
		Severity:      severity,
		ResponseModel: bazaarLinkSubModelID(identity),
		ExpectedModel: expectedModel,
	}
}

// bazaarLinkProbeEvidenceScore 完整验证采用 BazaarLink 顶层 score；快速验证采用声明模型的候选分。
func bazaarLinkProbeEvidenceScore(result bazaarLinkProbeResponse, expectedModel string, mode BazaarLinkProbeMode) int {
	if mode == BazaarLinkProbeModeQuick {
		if score, ok := bazaarLinkClaimedCandidateScore(result.IdentityAssessment, expectedModel); ok {
			return clampInt(int(math.Round(score)), 0, 100)
		}
	}
	return clampInt(result.Score, 0, 100)
}

// bazaarLinkClaimedCandidateScore 匹配声明模型在 V3 候选里的原始百分制分数。
func bazaarLinkClaimedCandidateScore(identity bazaarLinkIdentityAssessment, expectedModel string) (float64, bool) {
	target := strings.TrimSpace(expectedModel)
	if target == "" {
		target = strings.TrimSpace(identity.ClaimedModel)
	}
	if target == "" {
		return 0, false
	}
	for _, candidate := range bazaarLinkIdentityCandidates(identity) {
		if !bazaarLinkCandidateMatchesModel(candidate, target) {
			continue
		}
		if candidate.Score == nil {
			continue
		}
		return normalizeBazaarLinkCandidateScore(*candidate.Score)
	}
	if identity.SubModelMatchV3F != nil && bazaarLinkModelNameMatches(identity.SubModelMatchV3F.ModelID, target) {
		return normalizeBazaarLinkCandidateScore(identity.SubModelMatchV3F.Score)
	}
	return 0, false
}

func bazaarLinkClaimedCandidateScoreFromResponseBody(responseBody, expectedModel string) (float64, bool) {
	responseBody = strings.TrimSpace(responseBody)
	if responseBody == "" {
		return 0, false
	}
	var parsed bazaarLinkProbeResponse
	if err := json.Unmarshal([]byte(responseBody), &parsed); err == nil {
		if score, ok := bazaarLinkClaimedCandidateScore(parsed.IdentityAssessment, expectedModel); ok {
			return score, true
		}
	}
	target := strings.TrimSpace(expectedModel)
	identityStart := strings.Index(responseBody, `"identityAssessment"`)
	if identityStart < 0 {
		identityStart = 0
	}
	if target == "" {
		target = bazaarLinkStringPropertyFromText(responseBody, "claimedModel", identityStart)
	}
	if target == "" {
		return 0, false
	}
	for _, candidate := range bazaarLinkCandidatesFromResponseText(responseBody, identityStart) {
		if !bazaarLinkCandidateMatchesModel(candidate, target) {
			continue
		}
		if candidate.Score == nil {
			continue
		}
		return normalizeBazaarLinkCandidateScore(*candidate.Score)
	}
	return 0, false
}

func bazaarLinkIdentityCandidates(identity bazaarLinkIdentityAssessment) []bazaarLinkModelCandidate {
	candidates := make([]bazaarLinkModelCandidate, 0, len(identity.V3Candidates)+len(identity.Candidates))
	candidates = append(candidates, bazaarLinkCandidatesFromRaw(identity.V3)...)
	candidates = append(candidates, identity.V3Candidates...)
	candidates = append(candidates, identity.Candidates...)
	return candidates
}

func bazaarLinkCandidatesFromRaw(raw json.RawMessage) []bazaarLinkModelCandidate {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	var direct []bazaarLinkModelCandidate
	if err := json.Unmarshal(trimmed, &direct); err == nil && len(direct) > 0 {
		return direct
	}
	var group bazaarLinkCandidateGroup
	if err := json.Unmarshal(trimmed, &group); err != nil {
		return nil
	}
	candidates := make([]bazaarLinkModelCandidate, 0, len(group.Candidates)+len(group.TopCandidates)+len(group.Matches))
	candidates = append(candidates, group.Candidates...)
	candidates = append(candidates, group.TopCandidates...)
	candidates = append(candidates, group.Matches...)
	return candidates
}

func bazaarLinkCandidatesFromResponseText(source string, startIndex int) []bazaarLinkModelCandidate {
	v3Index := strings.Index(source[max(0, startIndex):], `"v3"`)
	if v3Index >= 0 {
		v3Index += max(0, startIndex)
	}
	candidatesStart := startIndex
	if v3Index >= 0 {
		candidatesStart = v3Index
	}
	if candidates := bazaarLinkCandidatesAfterProperty(source, "candidates", candidatesStart); len(candidates) > 0 {
		return candidates
	}
	return bazaarLinkCandidatesAfterProperty(source, "v3Candidates", startIndex)
}

func bazaarLinkCandidatesAfterProperty(source, property string, startIndex int) []bazaarLinkModelCandidate {
	arrayText := bazaarLinkJSONArrayAfterProperty(source, property, startIndex)
	if arrayText == "" {
		return nil
	}
	var candidates []bazaarLinkModelCandidate
	if err := json.Unmarshal([]byte(arrayText), &candidates); err != nil {
		return nil
	}
	return candidates
}

func bazaarLinkJSONArrayAfterProperty(source, property string, startIndex int) string {
	propertyIndex := strings.Index(source[max(0, startIndex):], `"`+property+`"`)
	if propertyIndex < 0 {
		return ""
	}
	propertyIndex += max(0, startIndex)
	colonIndex := strings.Index(source[propertyIndex+len(property)+2:], ":")
	if colonIndex < 0 {
		return ""
	}
	colonIndex += propertyIndex + len(property) + 2
	arrayStart := strings.Index(source[colonIndex+1:], "[")
	if arrayStart < 0 {
		return ""
	}
	arrayStart += colonIndex + 1
	arrayEnd := bazaarLinkBalancedJSONArrayEnd(source, arrayStart)
	if arrayEnd < 0 {
		return ""
	}
	return source[arrayStart : arrayEnd+1]
}

func bazaarLinkBalancedJSONArrayEnd(source string, arrayStart int) int {
	depth := 0
	inString := false
	escaped := false
	for index := arrayStart; index < len(source); index++ {
		char := source[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func bazaarLinkStringPropertyFromText(source, property string, startIndex int) string {
	propertyIndex := strings.Index(source[max(0, startIndex):], `"`+property+`"`)
	if propertyIndex < 0 {
		return ""
	}
	propertyIndex += max(0, startIndex)
	colonIndex := strings.Index(source[propertyIndex+len(property)+2:], ":")
	if colonIndex < 0 {
		return ""
	}
	colonIndex += propertyIndex + len(property) + 2
	valueIndex := colonIndex + 1
	for valueIndex < len(source) && (source[valueIndex] == ' ' || source[valueIndex] == '\n' || source[valueIndex] == '\r' || source[valueIndex] == '\t') {
		valueIndex++
	}
	if valueIndex >= len(source) || source[valueIndex] != '"' {
		return ""
	}
	escaped := false
	for index := valueIndex + 1; index < len(source); index++ {
		char := source[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			var value string
			if err := json.Unmarshal([]byte(source[valueIndex:index+1]), &value); err != nil {
				return ""
			}
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func bazaarLinkCandidateMatchesModel(candidate bazaarLinkModelCandidate, expectedModel string) bool {
	return bazaarLinkModelNameMatches(candidate.ModelID, expectedModel) || bazaarLinkModelNameMatches(candidate.DisplayName, expectedModel)
}

func bazaarLinkModelNameMatches(left, right string) bool {
	normalizedLeft := normalizeBazaarLinkModelName(left)
	normalizedRight := normalizeBazaarLinkModelName(right)
	if normalizedLeft == "" || normalizedRight == "" {
		return false
	}
	return normalizedLeft == normalizedRight || strings.HasSuffix(normalizedLeft, normalizedRight) || strings.HasSuffix(normalizedRight, normalizedLeft)
}

func normalizeBazaarLinkModelName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		value = value[slash+1:]
	}
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "")
	return replacer.Replace(value)
}

func normalizeBazaarLinkCandidateScore(score float64) (float64, bool) {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, false
	}
	if score <= 1 {
		score *= 100
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, true
}

// bazaarLinkIdentityStatusConfirmed 兼容 BazaarLink 页面返回的模型身份确认状态。
func bazaarLinkIdentityStatusConfirmed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed", "match", "matched":
		return true
	default:
		return false
	}
}

func bazaarLinkIdentityObserved(identity bazaarLinkIdentityAssessment) string {
	parts := []string{
		"status=" + strings.TrimSpace(identity.Status),
	}
	if identity.Confidence > 0 {
		parts = append(parts, fmt.Sprintf("confidence=%.2f", identity.Confidence))
	}
	if identity.PredictedFamily != "" {
		parts = append(parts, "family="+identity.PredictedFamily)
	}
	if modelID := bazaarLinkSubModelID(identity); modelID != "" {
		parts = append(parts, "v3f="+modelID)
	}
	if len(identity.RiskFlags) > 0 {
		parts = append(parts, "flags="+strings.Join(identity.RiskFlags, ","))
	}
	return strings.Join(parts, "; ")
}

func bazaarLinkSubModelID(identity bazaarLinkIdentityAssessment) string {
	if identity.SubModelMatchV3F == nil {
		return ""
	}
	return strings.TrimSpace(identity.SubModelMatchV3F.ModelID)
}

func bazaarLinkProbeSummary(result bazaarLinkProbeResponse) string {
	identity := result.IdentityAssessment
	return fmt.Sprintf("BazaarLink run=%s status=%s score=%d identity=%s v3f=%s flags=%s",
		strings.TrimSpace(result.RunID),
		strings.TrimSpace(result.Status),
		result.Score,
		strings.TrimSpace(identity.Status),
		bazaarLinkSubModelID(identity),
		strings.Join(identity.RiskFlags, ","),
	)
}

func bazaarLinkProbeLabel(mode BazaarLinkProbeMode) string {
	if mode == BazaarLinkProbeModeFull {
		return "BazaarLink 完整验证"
	}
	return "BazaarLink 快速验证"
}

func bazaarLinkProbeRequestBodyForDisplay(payload bazaarLinkProbePayload) string {
	safePayload := payload
	safePayload.APIKey = "<redacted>"
	data, _ := json.Marshal(safePayload)
	return truncateAccountProbeTranscript(string(data))
}

func bazaarLinkProbeResponseBodyForDisplay(body []byte) string {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return truncateAccountProbeTranscript(string(body))
	}
	redactBazaarLinkSecretFields(parsed)
	data, err := json.Marshal(parsed)
	if err != nil {
		return truncateAccountProbeTranscript(string(body))
	}
	return truncateAccountProbeTranscript(string(data))
}

func bazaarLinkProbeErrorMessageForDisplay(body []byte, fallback, apiKey string) string {
	message := truncateAccountProbeError([]byte(bazaarLinkProbeResponseBodyForDisplay(body)), fallback)
	return redactBazaarLinkKnownSecret(message, apiKey)
}

func redactBazaarLinkKnownSecret(text, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "<redacted>")
}

func redactBazaarLinkSecretFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			switch normalized {
			case "apikey", "authorization", "token", "accesstoken":
				typed[key] = "<redacted>"
			default:
				redactBazaarLinkSecretFields(child)
			}
		}
	case []any:
		for _, child := range typed {
			redactBazaarLinkSecretFields(child)
		}
	}
}

func bazaarLinkOpenAIBaseURL(baseURL string) string {
	responsesURL := buildOpenAIResponsesURL(baseURL)
	return strings.TrimSuffix(responsesURL, "/responses")
}

func intFromOptional(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

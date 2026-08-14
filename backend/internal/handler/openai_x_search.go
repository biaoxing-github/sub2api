package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	defaultGrokXSearchResults = 5
	maxGrokXSearchResults     = 20
)

type grokStandaloneXSearchRequest struct {
	Query                    string   `json:"query"`
	Input                    string   `json:"input"`
	MaxResults               *int     `json:"max_results"`
	AllowedXHandles          []string `json:"allowed_x_handles"`
	ExcludedXHandles         []string `json:"excluded_x_handles"`
	FromDate                 string   `json:"from_date"`
	ToDate                   string   `json:"to_date"`
	EnableImageUnderstanding *bool    `json:"enable_image_understanding"`
	EnableVideoUnderstanding *bool    `json:"enable_video_understanding"`
}

// XSearch 为 Grok 分组执行原生 x_search，并复用 Responses 的调度、并发、
// 账号切换、用量审计与网页搜索按次计费链路。
func (h *OpenAIGatewayHandler) XSearch(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	setOpenAIClientTransportHTTP(c)
	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group.Platform != service.PlatformGrok {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "X Search API is only available for Grok groups")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.x_search",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	var req grokStandaloneXSearchRequest
	if len(body) == 0 || json.Unmarshal(body, &req) != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		query = strings.TrimSpace(req.Input)
	}
	if query == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "query is required")
		return
	}
	req.Query = query
	maxResults := 0
	if req.MaxResults != nil {
		maxResults = *req.MaxResults
	}
	maxResults = normalizeGrokXSearchMaxResults(maxResults)
	requestedModel := service.ResolveGrokStandaloneSearchModel()
	reqLog = reqLog.With(zap.String("model", requestedModel))
	setOpsRequestContext(c, requestedModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	moderationBody, _ := json.Marshal(map[string]any{"messages": []map[string]any{{"role": "user", "content": query}}})
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, requestedModel, moderationBody); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, requestedModel)
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError
	switchCount := 0
	routingStart := time.Now()
	for {
		selection, err := h.gatewayService.SelectAccountWithLoadAwareness(c.Request.Context(), apiKey.GroupID, "", requestedModel, failedAccountIDs)
		if err != nil || selection == nil || selection.Account == nil {
			if failoverClientGone(c) {
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
			} else {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable")
			}
			return
		}

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		accountSlot := h.acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, &streamStarted, reqLog)
		if !accountSlot.Acquired {
			if accountSlot.SwitchAccount && accountSlot.FailoverErr != nil {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = accountSlot.FailoverErr
				if switchCount >= h.maxAccountSwitches {
					h.handleFailoverExhausted(c, accountSlot.FailoverErr, false)
					return
				}
				switchCount++
				continue
			}
			return
		}
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		mappedModel := requestedModel
		if channelMapping.Mapped && strings.TrimSpace(channelMapping.MappedModel) != "" {
			mappedModel = channelMapping.MappedModel
		}
		upstreamModel := account.GetMappedModel(mappedModel)
		if strings.TrimSpace(upstreamModel) == "" {
			upstreamModel = requestedModel
		}
		upstreamBody, buildErr := buildGrokXSearchResponsesBody(req, upstreamModel, maxResults)
		if buildErr != nil {
			if accountSlot.ReleaseFunc != nil {
				accountSlot.ReleaseFunc()
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", buildErr.Error())
			return
		}
		forwardStart := time.Now()
		responseBody, forwardErr := func() ([]byte, error) {
			if accountSlot.ReleaseFunc != nil {
				defer accountSlot.ReleaseFunc()
			}
			return h.gatewayService.DoGrokNativeResponsesJSON(c.Request.Context(), account, upstreamBody)
		}()
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, time.Since(forwardStart).Milliseconds())
		if forwardErr == nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
			result := &service.OpenAIForwardResult{
				RequestID:      "x_search:" + uuid.NewString(),
				Model:          "grok-x-search",
				UpstreamModel:  upstreamModel,
				WebSearchCalls: 1,
				Duration:       time.Since(requestStart),
			}
			h.recordXSearchUsage(c, apiKey, account, subscription, channelMapping, requestedModel, body, result, subject.UserID)
			c.JSON(http.StatusOK, gin.H{
				"query":       query,
				"results":     extractGrokXSearchSources(responseBody, maxResults),
				"provider":    "grok-native",
				"max_results": maxResults,
			})
			return
		}

		var failoverErr *service.UpstreamFailoverError
		if !errors.As(forwardErr, &failoverErr) {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			reqLog.Warn("openai_x_search.forward_failed", zap.Int64("account_id", account.ID), zap.Error(forwardErr))
			return
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
		h.gatewayService.RecordOpenAIAccountSwitch()
		failedAccountIDs[account.ID] = struct{}{}
		lastFailoverErr = failoverErr
		if switchCount >= h.maxAccountSwitches {
			h.handleFailoverExhausted(c, failoverErr, false)
			return
		}
		switchCount++
	}
}

func normalizeGrokXSearchMaxResults(maxResults int) int {
	if maxResults <= 0 {
		return defaultGrokXSearchResults
	}
	if maxResults > maxGrokXSearchResults {
		return maxGrokXSearchResults
	}
	return maxResults
}

func buildGrokXSearchResponsesBody(req grokStandaloneXSearchRequest, model string, maxResults int) ([]byte, error) {
	tool := map[string]any{"type": "x_search"}
	if len(req.AllowedXHandles) > 0 {
		tool["allowed_x_handles"] = req.AllowedXHandles
	}
	if len(req.ExcludedXHandles) > 0 {
		tool["excluded_x_handles"] = req.ExcludedXHandles
	}
	if value := strings.TrimSpace(req.FromDate); value != "" {
		tool["from_date"] = value
	}
	if value := strings.TrimSpace(req.ToDate); value != "" {
		tool["to_date"] = value
	}
	if req.EnableImageUnderstanding != nil {
		tool["enable_image_understanding"] = *req.EnableImageUnderstanding
	}
	if req.EnableVideoUnderstanding != nil {
		tool["enable_video_understanding"] = *req.EnableVideoUnderstanding
	}
	return json.Marshal(map[string]any{
		"model":       strings.TrimSpace(model),
		"input":       buildGrokXSearchPrompt(req.Query, maxResults),
		"tools":       []map[string]any{tool},
		"tool_choice": "required",
		"include":     []string{"x_search_call.action.sources"},
		"store":       false,
		"stream":      false,
	})
}

func buildGrokXSearchPrompt(query string, maxResults int) string {
	return fmt.Sprintf(`Search X for the user query below. Return ONLY valid JSON with this exact shape: {"results":[{"url":"https://...","title":"post or page title","snippet":"concise factual summary"}]}. Return at most %d unique results. Every URL must be an actual x_search source. Populate a non-empty title and snippet for every result. Do not wrap the JSON in markdown.

User query:
%s`, normalizeGrokXSearchMaxResults(maxResults), strings.TrimSpace(query))
}

// extractGrokXSearchSources 仅接受 x_search_call 声明过的 URL，并用模型返回的
// 结构化标题和摘要补全来源信息，避免把未引用链接当成搜索结果。
func extractGrokXSearchSources(body []byte, maxResults int) []websearch.SearchResult {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	maxResults = normalizeGrokXSearchMaxResults(maxResults)
	sources := make(map[string]websearch.SearchResult)
	var sourceOrder []string
	addSource := func(rawURL, title, snippet string) {
		key, ok := normalizeGrokXSearchURL(rawURL)
		if !ok {
			return
		}
		result, exists := sources[key]
		if !exists {
			result.URL = strings.TrimSpace(rawURL)
			sourceOrder = append(sourceOrder, key)
		}
		if result.Title == "" {
			result.Title = usableGrokXSearchTitle(title, result.URL)
		}
		if result.Snippet == "" {
			result.Snippet = strings.TrimSpace(snippet)
		}
		sources[key] = result
	}

	output := gjson.GetBytes(body, "output")
	output.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "x_search_call" {
			item.Get("action.sources").ForEach(func(_, source gjson.Result) bool {
				addSource(source.Get("url").String(), source.Get("title").String(), source.Get("snippet").String())
				return true
			})
		}
		if item.Get("type").String() == "message" {
			item.Get("content").ForEach(func(_, part gjson.Result) bool {
				part.Get("annotations").ForEach(func(_, annotation gjson.Result) bool {
					if annotation.Get("type").String() == "url_citation" || annotation.Get("type").String() == "web" {
						addSource(annotation.Get("url").String(), annotation.Get("title").String(), "")
					}
					return true
				})
				return true
			})
		}
		return true
	})

	var results []websearch.SearchResult
	seen := make(map[string]bool)
	output.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() != "message" {
			return true
		}
		item.Get("content").ForEach(func(_, part gjson.Result) bool {
			for _, result := range parseGrokXSearchStructuredResults(part.Get("text").String()) {
				key, ok := normalizeGrokXSearchURL(result.URL)
				source, allowed := sources[key]
				if !ok || !allowed || seen[key] || len(results) >= maxResults {
					continue
				}
				seen[key] = true
				result.URL = source.URL
				result.Title = usableGrokXSearchTitle(result.Title, result.URL)
				if result.Title == "" {
					result.Title = source.Title
				}
				if strings.TrimSpace(result.Snippet) == "" {
					result.Snippet = source.Snippet
				}
				results = append(results, result)
			}
			return true
		})
		return len(results) < maxResults
	})
	for _, key := range sourceOrder {
		if len(results) >= maxResults {
			break
		}
		if seen[key] {
			continue
		}
		result := sources[key]
		if result.Title == "" {
			result.Title = grokXSearchTitleFromURL(result.URL)
		}
		results = append(results, result)
	}
	return results
}

func parseGrokXSearchStructuredResults(text string) []websearch.SearchResult {
	start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return nil
	}
	var payload struct {
		Results []websearch.SearchResult `json:"results"`
	}
	if json.Unmarshal([]byte(text[start:end+1]), &payload) != nil {
		return nil
	}
	return payload.Results
}

func normalizeGrokXSearchURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), true
}

func usableGrokXSearchTitle(title, rawURL string) string {
	title = strings.TrimSpace(title)
	if title == "" || title == rawURL {
		return ""
	}
	if _, err := strconv.Atoi(title); err == nil {
		return ""
	}
	return title
}

func grokXSearchTitleFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
}

func (h *OpenAIGatewayHandler) recordXSearchUsage(
	c *gin.Context,
	apiKey *service.APIKey,
	account *service.Account,
	subscription *service.UserSubscription,
	channelMapping service.ChannelMappingResult,
	requestedModel string,
	body []byte,
	result *service.OpenAIForwardResult,
	userID int64,
) {
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	sessionID := service.ExtractClientSessionID(c)
	requestPayloadHash := service.HashUsageRequestPayload(body)
	h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			SessionID:          sessionID,
			RequestPayloadHash: requestPayloadHash,
			APIKeyService:      h.apiKeyService,
			ChannelUsageFields: channelMapping.ToUsageFields(requestedModel, result.UpstreamModel),
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.x_search"),
				zap.Int64("user_id", userID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Int64("account_id", account.ID),
			).Error("openai_x_search.record_usage_failed", zap.Error(err))
		}
	})
}

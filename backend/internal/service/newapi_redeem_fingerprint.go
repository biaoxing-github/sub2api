package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// newAPIRedeemBrowserProfile 描述账号长期绑定的浏览器客户端身份。
// ID 会写入本地状态文件；TLS 模板、User-Agent 和客户端提示头必须成套变化。
type newAPIRedeemBrowserProfile struct {
	ID              string
	Name            string
	UserAgent       string
	AcceptLanguage  string
	SecCHUA         string
	SecCHUAMobile   string
	SecCHUAPlatform string
	TLSProfile      tlsfingerprint.Profile
}

// newAPIRedeemBrowserProfiles 与 openai_tls_profile.go 的十套固定 uTLS 模板保持相同顺序。
// 一个任务最多使用十个互不相同的身份；更多账号按固定顺序复用，且同账号不会随请求漂移。
var newAPIRedeemBrowserProfiles = [...]newAPIRedeemBrowserProfile{
	{ID: "chrome-100-windows", Name: "Chrome 100 / Windows", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36", AcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.8", SecCHUA: `"Chromium";v="100", "Google Chrome";v="100", "Not:A-Brand";v="99"`, SecCHUAMobile: "?0", SecCHUAPlatform: `"Windows"`},
	{ID: "safari-ios-12", Name: "Safari / iOS 12.1", UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 12_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/12.0 Mobile/15E148 Safari/604.1", AcceptLanguage: "zh-CN,zh;q=0.9,en-US;q=0.8"},
	{ID: "firefox-105-windows", Name: "Firefox 105 / Windows", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:105.0) Gecko/20100101 Firefox/105.0", AcceptLanguage: "zh-CN,zh;q=0.8,en-US;q=0.5,en;q=0.3"},
	{ID: "firefox-120-linux", Name: "Firefox 120 / Linux", UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0", AcceptLanguage: "en-US,en;q=0.8,zh-CN;q=0.5"},
	{ID: "safari-16-macos", Name: "Safari 16 / macOS", UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15", AcceptLanguage: "zh-CN,zh-Hans;q=0.9,en;q=0.8"},
	{ID: "safari-ios-14", Name: "Safari / iOS 14", UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1", AcceptLanguage: "zh-CN,zh;q=0.9,en-US;q=0.8"},
	{ID: "okhttp-android-11", Name: "OkHttp / Android 11", UserAgent: "okhttp/4.10.0", AcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.8"},
	{ID: "edge-85-windows", Name: "Edge 85 / Windows", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.83 Safari/537.36 Edg/85.0.564.44", AcceptLanguage: "en-US,en;q=0.9,zh-CN;q=0.8", SecCHUA: `"Chromium";v="85", "Microsoft Edge";v="85", ";Not A Brand";v="99"`, SecCHUAMobile: "?0", SecCHUAPlatform: `"Windows"`},
	{ID: "360-browser-7", Name: "360 Browser 7.5 / Windows", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36 QIHU 360SE", AcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.7"},
	{ID: "qq-browser-11", Name: "QQ Browser 11.1 / Android", UserAgent: "Mozilla/5.0 (Linux; U; Android 11; zh-cn) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/87.0.4280.101 Mobile Safari/537.36 MQQBrowser/11.1", AcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.6", SecCHUAMobile: "?1", SecCHUAPlatform: `"Android"`},
}

func init() {
	for index := range newAPIRedeemBrowserProfiles {
		newAPIRedeemBrowserProfiles[index].TLSProfile = builtInOpenAICodexTLSFingerprintProfiles[index]
	}
}

// newAPIRedeemBrowserProfileByID 返回持久化 ID 对应的完整客户端身份。
func newAPIRedeemBrowserProfileByID(profileID string) (newAPIRedeemBrowserProfile, bool) {
	for _, profile := range newAPIRedeemBrowserProfiles {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return newAPIRedeemBrowserProfile{}, false
}

// allocateNewAPIRedeemBrowserProfileID 优先分配当前账号列表尚未使用的浏览器身份。
func allocateNewAPIRedeemBrowserProfileID(accounts []newAPIRedeemStoredAccount) string {
	used := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if _, ok := newAPIRedeemBrowserProfileByID(account.BrowserProfileID); ok {
			used[account.BrowserProfileID] = struct{}{}
		}
	}
	for _, profile := range newAPIRedeemBrowserProfiles {
		if _, exists := used[profile.ID]; !exists {
			return profile.ID
		}
	}
	return newAPIRedeemBrowserProfiles[len(accounts)%len(newAPIRedeemBrowserProfiles)].ID
}

// ensureNewAPIRedeemBrowserProfileAssignments 为旧状态补齐账号身份，并修复前十个账号的重复分配。
func ensureNewAPIRedeemBrowserProfileAssignments(accounts []newAPIRedeemStoredAccount) bool {
	changed := false
	used := make(map[string]struct{}, len(accounts))
	for index := range accounts {
		profileID := accounts[index].BrowserProfileID
		_, valid := newAPIRedeemBrowserProfileByID(profileID)
		_, duplicate := used[profileID]
		if !valid || (duplicate && index < len(newAPIRedeemBrowserProfiles)) {
			profileID = ""
			for _, candidate := range newAPIRedeemBrowserProfiles {
				if _, exists := used[candidate.ID]; !exists {
					profileID = candidate.ID
					break
				}
			}
			if profileID == "" {
				profileID = newAPIRedeemBrowserProfiles[index%len(newAPIRedeemBrowserProfiles)].ID
			}
			accounts[index].BrowserProfileID = profileID
			changed = true
		}
		used[profileID] = struct{}{}
	}
	return changed
}

// newAPIRedeemAccountHTTPClient 构造并缓存账号专属连接池。
// HTTPS 请求使用账号绑定的 uTLS 模板；HTTP 测试服务器仍复用注入的 RoundTripper。
func (s *NewAPIRedeemService) newAPIRedeemAccountHTTPClient(account newAPIRedeemStoredAccount) (*http.Client, newAPIRedeemBrowserProfile, error) {
	profile, ok := newAPIRedeemBrowserProfileByID(account.BrowserProfileID)
	if !ok {
		return nil, newAPIRedeemBrowserProfile{}, fmt.Errorf("账号 %s 的浏览器指纹配置无效", account.UserID)
	}
	cacheKey := account.ID + "|" + profile.ID
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if client := s.accountClients[cacheKey]; client != nil {
		return client, profile, nil
	}

	client := *s.httpClient
	parsedBaseURL, err := url.Parse(s.baseURL)
	if err != nil {
		return nil, newAPIRedeemBrowserProfile{}, fmt.Errorf("解析 NewAPI 地址: %w", err)
	}
	if strings.EqualFold(parsedBaseURL.Scheme, "https") {
		baseTransport := s.httpClient.Transport
		if baseTransport == nil {
			baseTransport = http.DefaultTransport
		}
		transport, ok := baseTransport.(*http.Transport)
		if !ok {
			return nil, newAPIRedeemBrowserProfile{}, fmt.Errorf("NewAPI HTTPS 客户端传输类型不支持账号级 TLS 指纹: %T", baseTransport)
		}
		clonedTransport := transport.Clone()
		clonedTransport.ForceAttemptHTTP2 = false
		clonedTransport.DialTLSContext = tlsfingerprint.NewDialer(&profile.TLSProfile, clonedTransport.DialContext).DialTLSContext
		client.Transport = clonedTransport
	}
	s.accountClients[cacheKey] = &client
	return &client, profile, nil
}

// applyNewAPIRedeemBrowserHeaders 为请求写入与 TLS 模板成套的浏览器头。
func applyNewAPIRedeemBrowserHeaders(req *http.Request, baseURL string, profile newAPIRedeemBrowserProfile) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", profile.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", profile.AcceptLanguage)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origin := parsed.Scheme + "://" + parsed.Host
		req.Header.Set("Origin", origin)
		req.Header.Set("Referer", origin+"/")
	}
	if profile.SecCHUA != "" {
		req.Header.Set("Sec-CH-UA", profile.SecCHUA)
	}
	if profile.SecCHUAMobile != "" {
		req.Header.Set("Sec-CH-UA-Mobile", profile.SecCHUAMobile)
	}
	if profile.SecCHUAPlatform != "" {
		req.Header.Set("Sec-CH-UA-Platform", profile.SecCHUAPlatform)
	}
}

// closeNewAPIRedeemAccountIdleConnections 丢弃旧出口 IP 上的全部账号空闲连接。
func (s *NewAPIRedeemService) closeNewAPIRedeemAccountIdleConnections() {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	for _, client := range s.accountClients {
		if client != nil {
			client.CloseIdleConnections()
		}
	}
}

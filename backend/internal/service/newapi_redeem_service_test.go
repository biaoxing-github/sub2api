package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func TestNewAPIRedeemServiceRefreshesAccountWithoutExposingAccessKey(t *testing.T) {
	t.Parallel()
	var (
		mu          sync.Mutex
		fullKeyHits int
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer source-access-key", request.Header.Get("Authorization"))
		require.Equal(t, "14690", request.Header.Get("New-Api-User"))
		switch request.URL.Path {
		case "/api/user/self":
			_, _ = writer.Write([]byte(`{"success":true,"data":{"username":"alice","email":"alice@example.com","group":"default","quota":123456,"used_quota":456}}`))
		case "/api/token/":
			_, _ = writer.Write([]byte(`{"success":true,"data":{"items":[{"id":7,"name":"primary","group":"default","key":"sk-partial-list"}]}}`))
		case "/api/token/7/key":
			mu.Lock()
			fullKeyHits++
			mu.Unlock()
			require.Equal(t, http.MethodPost, request.Method)
			_, _ = writer.Write([]byte(`{"success":true,"data":{"key":"sk-generated-secret"}}`))
		case "/api/user/self/groups":
			_, _ = writer.Write([]byte(`{"success":true,"data":["default","pro"]}`))
		default:
			_, _ = writer.Write([]byte(`{"success":false,"message":"unsupported"}`))
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client()})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "14690", AccessKey: "source-access-key"}})
	require.NoError(t, err)
	require.Len(t, accounts, 1)

	refreshed, err := service.RefreshAccount(context.Background(), accounts[0].ID)
	require.NoError(t, err)
	require.Equal(t, "alice", refreshed.Username)
	require.Equal(t, "alice@example.com", refreshed.Email)
	require.Equal(t, "123456", refreshed.Quota)
	require.Equal(t, "sour***-key", refreshed.AccessKeyMasked)
	require.Equal(t, "sk-p***list", refreshed.APIKeys[0].MaskedKey)
	require.Equal(t, []string{"default", "pro"}, refreshed.AvailableGroups)
	mu.Lock()
	require.Equal(t, 0, fullKeyHits, "刷新账号不应逐个请求完整 API Key")
	mu.Unlock()

	secret, err := service.RevealAPIKey(context.Background(), accounts[0].ID, 7)
	require.NoError(t, err)
	require.Equal(t, "sk-generated-secret", secret.Key)
	mu.Lock()
	require.Equal(t, 1, fullKeyHits)
	mu.Unlock()

	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	payload, err := json.Marshal(overview)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "source-access-key")
	require.NotContains(t, string(payload), "sk-generated-secret")
}

func TestNewAPIRedeemServiceStopsAccountAfterSuccessfulRedemption(t *testing.T) {
	t.Parallel()
	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/user/topup", request.URL.Path)
		require.Equal(t, "Bearer access-key", request.Header.Get("Authorization"))
		require.Equal(t, "14744", request.Header.Get("New-Api-User"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		mu.Lock()
		requests = append(requests, payload["key"])
		mu.Unlock()
		if payload["key"] == "success-code" {
			_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"success":false,"message":"兑换码无效"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "14744", AccessKey: "access-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "codes.txt", []byte("failed-code\nsuccess-code\nunused-code\n"))
	require.NoError(t, err)

	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 2, completed.Attempts)
	require.Equal(t, 1, completed.Successes)
	require.Len(t, completed.Logs, 2)
	require.Equal(t, []string{"invalid_code", "redeemed"}, []string{completed.Logs[0].Result, completed.Logs[1].Result})
	for _, log := range completed.Logs {
		require.True(t, log.RemovedFromFile)
	}
	mu.Lock()
	require.Equal(t, []string{"failed-code", "success-code"}, requests)
	mu.Unlock()
	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, overview.Files[0].CodeCount)

	repeated, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	repeatedCompleted := waitForNewAPIRedeemRun(t, service, repeated.ID)
	require.Equal(t, 0, repeatedCompleted.Attempts)
	require.Equal(t, 0, repeatedCompleted.Successes)
}

func TestNewAPIRedeemServiceTreatsAlreadyRedeemedAsSuccess(t *testing.T) {
	t.Parallel()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/user/topup", request.URL.Path)
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		requests = append(requests, payload["key"])
		_, _ = writer.Write([]byte(`{"success":false,"message":"该分组兑换码每人限兑 1 次，您已达到上限"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "14755", AccessKey: "access-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "already.txt", []byte("already-code-1\nalready-code-2\n"))
	require.NoError(t, err)
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, 1, completed.Attempts)
	require.Equal(t, 1, completed.Successes)
	require.Equal(t, []string{"already-code-1"}, requests)
	require.Equal(t, "already_redeemed", completed.Logs[0].Result)
	require.True(t, strings.Contains(completed.Logs[0].Message, "已达到上限"))
	require.False(t, completed.Logs[0].RemovedFromFile)

	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, overview.Accounts, 1)
	require.Len(t, overview.Accounts[0].RedeemedCodes, 1)
	require.Equal(t, "already.txt", overview.Accounts[0].RedeemedCodes[0].FileName)
	require.Equal(t, file.ID, overview.Accounts[0].RedeemedCodes[0].FileID)
	require.Equal(t, 2, overview.Files[0].CodeCount)

	repeated, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	repeatedCompleted := waitForNewAPIRedeemRun(t, service, repeated.ID)
	require.Equal(t, 0, repeatedCompleted.Attempts)
}

func TestNewAPIRedeemServicePrioritizesAlreadyRedeemedCodeForUnmarkedAccount(t *testing.T) {
	t.Parallel()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		requests = append(requests, payload["key"])
		if payload["key"] == "shared-code" {
			_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"success":false,"message":"兑换码无效"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{
		{UserID: "marked-account", AccessKey: "marked-key"},
		{UserID: "pending-account", AccessKey: "pending-key"},
	})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "priority.txt", []byte("ordinary-code\nshared-code\n"))
	require.NoError(t, err)

	service.mu.Lock()
	service.state.Redemptions = append(service.state.Redemptions, newAPIRedeemRedemption{
		FileID: file.ID, AccountID: accounts[0].ID, Code: "shared-code", CodeHash: newAPIRedeemCodeHash(file.ID, "shared-code"), Result: "already_redeemed", RedeemedAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, service.saveStateLocked())
	service.mu.Unlock()

	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{
		FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID, accounts[1].ID},
	})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, []string{"shared-code"}, requests)
	require.Equal(t, "redeemed", completed.Logs[0].Result)
	require.True(t, completed.Logs[0].RemovedFromFile)

	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, overview.Files[0].CodeCount)
}

func TestNewAPIRedeemServiceRequeuesAlreadyRedeemedCodeForNextAccount(t *testing.T) {
	t.Parallel()
	var (
		mu          sync.Mutex
		sharedUsers []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		if payload["key"] != "shared-code" {
			_, _ = writer.Write([]byte(`{"success":false,"message":"兑换码无效"}`))
			return
		}
		mu.Lock()
		sharedUsers = append(sharedUsers, request.Header.Get("New-Api-User"))
		hit := len(sharedUsers)
		mu.Unlock()
		if hit == 1 {
			_, _ = writer.Write([]byte(`{"success":false,"message":"该分组兑换码每人限兑 1 次，您已达到上限"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{
		{UserID: "next-account-1", AccessKey: "key-1"},
		{UserID: "next-account-2", AccessKey: "key-2"},
	})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "requeue.txt", []byte("shared-code\nordinary-code\n"))
	require.NoError(t, err)

	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{
		FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID, accounts[1].ID},
	})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 3, completed.Attempts)
	require.Equal(t, 2, completed.Successes)

	mu.Lock()
	require.Len(t, sharedUsers, 2)
	require.NotEqual(t, sharedUsers[0], sharedUsers[1])
	mu.Unlock()
	require.Equal(t, "redeemed", completed.Logs[len(completed.Logs)-1].Result)
	require.True(t, completed.Logs[len(completed.Logs)-1].RemovedFromFile)
}

func TestNewAPIRedeemServiceFlushesRedeemedCodeBeforeNextFile(t *testing.T) {
	t.Parallel()
	secondStarted := make(chan struct{})
	releaseSecond := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		if payload["key"] == "success-code" {
			_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
			return
		}
		close(secondStarted)
		<-releaseSecond
		_, _ = writer.Write([]byte(`{"success":false,"message":"该分组兑换码每人限兑 1 次，您已达到上限"}`))
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(releaseSecond) })

	rootDir := t.TempDir()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir, BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "flush-account", AccessKey: "flush-key"}})
	require.NoError(t, err)
	firstFile, err := service.SaveVoucherContent(context.Background(), "success.txt", []byte("success-code\n"))
	require.NoError(t, err)
	secondFile, err := service.SaveVoucherContent(context.Background(), "blocking.txt", []byte("blocking-code\n"))
	require.NoError(t, err)

	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{
		FileIDs: []string{firstFile.ID, secondFile.ID}, AccountIDs: []string{accounts[0].ID},
	})
	require.NoError(t, err)
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("第二个文件未在预期时间内开始")
	}
	content, err := os.ReadFile(filepath.Join(rootDir, newAPIRedeemUploadsDirName, firstFile.ID+".txt"))
	require.NoError(t, err)
	require.Empty(t, parseNewAPIRedeemCodes(content))

	releaseOnce.Do(func() { close(releaseSecond) })
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
}

func TestNewAPIRedeemServiceProcessesFilesSequentiallyAndSkipsMarkedPairs(t *testing.T) {
	t.Parallel()
	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		mu.Lock()
		requests = append(requests, payload["key"])
		mu.Unlock()
		_, _ = writer.Write([]byte(`{"success":false,"message":"该分组兑换码每人限兑 1 次，您已达到上限"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{
		{UserID: "sequential-1", AccessKey: "key-1"},
		{UserID: "sequential-2", AccessKey: "key-2"},
	})
	require.NoError(t, err)
	firstFile, err := service.SaveVoucherContent(context.Background(), "first.txt", []byte("first-code-1\nfirst-code-2\n"))
	require.NoError(t, err)
	secondFile, err := service.SaveVoucherContent(context.Background(), "second.txt", []byte("second-code-1\nsecond-code-2\n"))
	require.NoError(t, err)

	service.mu.Lock()
	service.state.Redemptions = append(service.state.Redemptions, newAPIRedeemRedemption{
		FileID: firstFile.ID, AccountID: accounts[0].ID, CodeHash: newAPIRedeemCodeHash(firstFile.ID, "previous-code"), Result: "already_redeemed", RedeemedAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, service.saveStateLocked())
	service.mu.Unlock()

	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{
		FileIDs: []string{firstFile.ID, secondFile.ID}, AccountIDs: []string{accounts[0].ID, accounts[1].ID},
	})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 3, completed.Attempts)
	require.Equal(t, 3, completed.Successes)

	mu.Lock()
	require.Len(t, requests, 3)
	require.True(t, strings.HasPrefix(requests[0], "first-code-"))
	require.True(t, strings.HasPrefix(requests[1], "second-code-"))
	require.True(t, strings.HasPrefix(requests[2], "second-code-"))
	mu.Unlock()
	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	for _, account := range overview.Accounts {
		require.Len(t, account.RedeemedCodes, 2)
	}
}

func TestNewAPIRedeemServiceUsesDistinctStableBrowserFingerprintsInParallel(t *testing.T) {
	t.Parallel()
	var (
		mu            sync.Mutex
		inFlight      int
		maxInFlight   int
		requestCounts = map[string]int{}
		userAgents    = map[string]string{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/user/topup" {
			http.NotFound(writer, request)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读取请求体: %v", err)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("解析请求体: %v", err)
			return
		}
		userID := request.Header.Get("New-Api-User")
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		requestCounts[payload["key"]]++
		userAgents[userID] = request.Header.Get("User-Agent")
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		inFlight--
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":false,"message":"兑换码无效"}`))
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), RequestInterval: time.Millisecond})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{
		{UserID: "parallel-1", AccessKey: "key-1"},
		{UserID: "parallel-2", AccessKey: "key-2"},
		{UserID: "parallel-3", AccessKey: "key-3"},
	})
	require.NoError(t, err)
	codes := make([]string, 24)
	for index := range codes {
		codes[index] = fmt.Sprintf("parallel-code-%02d", index)
	}
	file, err := service.SaveVoucherContent(context.Background(), "parallel.txt", []byte(strings.Join(codes, "\n")+"\n"))
	require.NoError(t, err)
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: accountIDs})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, len(codes), completed.TotalPairs)
	require.Equal(t, len(codes), completed.CompletedPairs)
	require.Equal(t, len(codes), completed.Attempts)

	mu.Lock()
	require.GreaterOrEqual(t, maxInFlight, 2)
	require.Len(t, userAgents, len(accounts))
	require.Len(t, requestCounts, len(codes))
	for _, count := range requestCounts {
		require.Equal(t, 1, count)
	}
	uniqueUserAgents := map[string]struct{}{}
	for _, userAgent := range userAgents {
		require.NotEmpty(t, userAgent)
		uniqueUserAgents[userAgent] = struct{}{}
	}
	require.Len(t, uniqueUserAgents, len(accounts))
	mu.Unlock()
	for _, log := range completed.Logs {
		require.NotEmpty(t, log.BrowserFingerprint)
	}
}

func TestNewAPIRedeemServicePersistsTenDistinctBrowserFingerprintAssignments(t *testing.T) {
	t.Parallel()
	rootDir := t.TempDir()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	inputs := make([]NewAPIRedeemAccountInput, len(newAPIRedeemBrowserProfiles))
	for index := range inputs {
		inputs[index] = NewAPIRedeemAccountInput{UserID: fmt.Sprintf("fingerprint-%02d", index), AccessKey: fmt.Sprintf("key-%02d", index)}
	}
	_, err := service.ImportAccounts(context.Background(), inputs)
	require.NoError(t, err)
	before, err := service.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, before.Accounts, len(newAPIRedeemBrowserProfiles))
	assignments := map[string]string{}
	uniqueNames := map[string]struct{}{}
	for _, account := range before.Accounts {
		require.NotEmpty(t, account.BrowserFingerprint)
		assignments[account.UserID] = account.BrowserFingerprint
		uniqueNames[account.BrowserFingerprint] = struct{}{}
	}
	require.Len(t, uniqueNames, len(newAPIRedeemBrowserProfiles))

	restarted := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	after, err := restarted.Overview(context.Background())
	require.NoError(t, err)
	for _, account := range after.Accounts {
		require.Equal(t, assignments[account.UserID], account.BrowserFingerprint)
	}

	uniqueTLSProfiles := map[string]struct{}{}
	for _, profile := range newAPIRedeemBrowserProfiles {
		uniqueTLSProfiles[tlsfingerprint.ProfileCacheKey(&profile.TLSProfile)] = struct{}{}
	}
	require.Len(t, uniqueTLSProfiles, len(newAPIRedeemBrowserProfiles))
}

func TestNewAPIRedeemRequestSpacingSmoothsAggregateConcurrency(t *testing.T) {
	t.Parallel()
	require.Equal(t, 200*time.Millisecond, newAPIRedeemRequestSpacing(2*time.Second, 10))
	require.Equal(t, 2*time.Second, newAPIRedeemRequestSpacing(2*time.Second, 1))
	require.Equal(t, time.Duration(0), newAPIRedeemRequestSpacing(0, 10))
}

func TestNewAPIRedeemServiceSwitchesToUniqueNodeAndExitIPThenRetriesSameCode(t *testing.T) {
	t.Parallel()
	var (
		mu            sync.Mutex
		currentNode   = "node-a"
		topupRequests []string
		switchedNodes []string
		groupDelays   int
		ipHitsByNode  = map[string]int{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case request.URL.Path == "/api/user/topup":
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			topupRequests = append(topupRequests, payload["key"])
			if len(topupRequests) == 1 {
				writer.WriteHeader(http.StatusTooManyRequests)
				_, _ = writer.Write([]byte(`{"success":false,"message":"请求过于频繁"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
		case strings.HasPrefix(request.URL.Path, "/proxies/") && strings.HasSuffix(request.URL.Path, "/delay"):
			_, _ = writer.Write([]byte(`{"delay":20}`))
		case request.URL.Path == "/group/test-group/delay":
			groupDelays++
			_, _ = writer.Write([]byte(`{"node-a":30,"node-b":10,"node-c":20}`))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodGet:
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"name":"test-group","type":"Selector","now":%q,"all":["node-a","node-b","node-c"]}`, currentNode)))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodPut:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			currentNode = payload["name"]
			switchedNodes = append(switchedNodes, currentNode)
			writer.WriteHeader(http.StatusNoContent)
		case request.URL.Path == "/proxies":
			_, _ = writer.Write([]byte(`{"proxies":{"node-a":{"type":"Trojan"},"node-b":{"type":"Trojan"},"node-c":{"type":"Trojan"}}}`))
		case request.URL.Path == "/ip":
			ipHitsByNode[currentNode]++
			ip := "1.1.1.1"
			if currentNode == "node-b" && ipHitsByNode[currentNode] >= 2 {
				ip = "2.2.2.2"
			}
			_, _ = writer.Write([]byte(ip))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:                  t.TempDir(),
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		RequestInterval:          time.Millisecond,
		MihomoControllerURL:      server.URL,
		MihomoControllerSecret:   "test-secret",
		MihomoSelectorGroup:      "test-group",
		MihomoDelayURL:           server.URL + "/status",
		ExitIPURL:                server.URL + "/ip",
		MihomoHTTPClient:         server.Client(),
		MihomoSwitchPollInterval: time.Millisecond,
		MihomoSwitchPollAttempts: 2,
	})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "14777", AccessKey: "access-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "rate-limit.txt", []byte("same-code\n"))
	require.NoError(t, err)
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 2, completed.Attempts)
	require.Equal(t, 1, completed.SwitchCount)
	require.Equal(t, "node-b", completed.CurrentNode)
	require.Equal(t, "2.2.2.2", completed.CurrentExitIP)
	require.Equal(t, []string{"same-code", "same-code"}, topupRequests)
	require.Equal(t, []string{"node-b"}, switchedNodes)
	require.Equal(t, 1, groupDelays)
	require.Equal(t, "node-a", completed.Logs[0].Node)
	require.Equal(t, "1.1.1.1", completed.Logs[0].ExitIP)
	require.Equal(t, "node-b", completed.Logs[0].SwitchedNode)
	require.Equal(t, "2.2.2.2", completed.Logs[0].SwitchedExitIP)
	require.Equal(t, "node-b", completed.Logs[1].Node)
	require.Equal(t, "2.2.2.2", completed.Logs[1].ExitIP)
}

// TestNewAPIRedeemServicePausesAndRetriesCurrentFileAfterExitExhaustion 验证出口耗尽只暂停当前任务，并在冷却后重试同一码。
func TestNewAPIRedeemServicePausesAndRetriesCurrentFileAfterExitExhaustion(t *testing.T) {
	t.Parallel()
	var (
		mu             sync.Mutex
		currentNode    = "node-a"
		topupRequests  []string
		inspectionHits int
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case request.URL.Path == "/api/user/topup":
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			topupRequests = append(topupRequests, payload["key"])
			if len(topupRequests) == 1 {
				writer.WriteHeader(http.StatusTooManyRequests)
				_, _ = writer.Write([]byte(`{"success":false,"message":"请求过于频繁"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
		case strings.HasPrefix(request.URL.Path, "/proxies/") && strings.HasSuffix(request.URL.Path, "/delay"):
			_, _ = writer.Write([]byte(`{"delay":20}`))
		case request.URL.Path == "/group/test-group/delay":
			_, _ = writer.Write([]byte(`{"node-a":20,"node-b":20}`))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodGet:
			inspectionHits++
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"name":"test-group","type":"Selector","now":%q,"all":["node-a","node-b"]}`, currentNode)))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodPut:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			currentNode = payload["name"]
			writer.WriteHeader(http.StatusNoContent)
		case request.URL.Path == "/proxies":
			_, _ = writer.Write([]byte(`{"proxies":{"node-a":{"type":"Trojan"},"node-b":{"type":"Trojan"}}}`))
		case request.URL.Path == "/ip":
			_, _ = writer.Write([]byte("1.1.1.1"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:                  t.TempDir(),
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		RequestInterval:          time.Millisecond,
		NetworkRetryDelay:        100 * time.Millisecond,
		MihomoControllerURL:      server.URL,
		MihomoSelectorGroup:      "test-group",
		MihomoDelayURL:           server.URL + "/status",
		ExitIPURL:                server.URL + "/ip",
		MihomoHTTPClient:         server.Client(),
		MihomoSwitchPollInterval: time.Millisecond,
		MihomoSwitchPollAttempts: 1,
	})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "retry-user", AccessKey: "retry-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "retry.txt", []byte("retry-code\n"))
	require.NoError(t, err)
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		current, getErr := service.GetRun(context.Background(), run.ID)
		return getErr == nil && current.Status == "running" && strings.Contains(current.Message, "暂停 100ms 后重试当前文件")
	}, time.Second, 5*time.Millisecond)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 2, completed.Attempts)
	mu.Lock()
	require.Equal(t, []string{"retry-code", "retry-code"}, topupRequests)
	require.GreaterOrEqual(t, inspectionHits, 2)
	mu.Unlock()
}

// TestNewAPIRedeemServiceCancelsImmediatelyDuringNetworkRetryDelay 验证停止操作会立即打断出口冷却计时器。
func TestNewAPIRedeemServiceCancelsImmediatelyDuringNetworkRetryDelay(t *testing.T) {
	t.Parallel()
	var (
		mu            sync.Mutex
		currentNode   = "node-a"
		topupRequests int
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case request.URL.Path == "/api/user/topup":
			topupRequests++
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"success":false,"message":"请求过于频繁"}`))
		case strings.HasPrefix(request.URL.Path, "/proxies/") && strings.HasSuffix(request.URL.Path, "/delay"):
			_, _ = writer.Write([]byte(`{"delay":20}`))
		case request.URL.Path == "/group/test-group/delay":
			_, _ = writer.Write([]byte(`{"node-a":20,"node-b":20}`))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodGet:
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"name":"test-group","type":"Selector","now":%q,"all":["node-a","node-b"]}`, currentNode)))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodPut:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			currentNode = payload["name"]
			writer.WriteHeader(http.StatusNoContent)
		case request.URL.Path == "/proxies":
			_, _ = writer.Write([]byte(`{"proxies":{"node-a":{"type":"Trojan"},"node-b":{"type":"Trojan"}}}`))
		case request.URL.Path == "/ip":
			_, _ = writer.Write([]byte("1.1.1.1"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:                  t.TempDir(),
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		RequestInterval:          time.Millisecond,
		NetworkRetryDelay:        5 * time.Second,
		MihomoControllerURL:      server.URL,
		MihomoSelectorGroup:      "test-group",
		MihomoDelayURL:           server.URL + "/status",
		ExitIPURL:                server.URL + "/ip",
		MihomoHTTPClient:         server.Client(),
		MihomoSwitchPollInterval: time.Millisecond,
		MihomoSwitchPollAttempts: 1,
	})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "cancel-user", AccessKey: "cancel-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "cancel.txt", []byte("cancel-code\n"))
	require.NoError(t, err)
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: []string{accounts[0].ID}})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		current, getErr := service.GetRun(context.Background(), run.ID)
		return getErr == nil && strings.Contains(current.Message, "暂停 5s 后重试当前文件")
	}, time.Second, 5*time.Millisecond)

	cancelledAt := time.Now()
	_, err = service.CancelRun(context.Background(), run.ID)
	require.NoError(t, err)
	cancelled := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "cancelled", cancelled.Status)
	require.Less(t, time.Since(cancelledAt), time.Second)
	mu.Lock()
	require.Equal(t, 1, topupRequests)
	mu.Unlock()
}

func TestNewAPIRedeemServiceCoordinatesOneNetworkSwitchForConcurrentRateLimits(t *testing.T) {
	t.Parallel()
	const workerCount = 3
	var (
		mu            sync.Mutex
		currentNode   = "node-a"
		firstWave     = make(chan struct{})
		firstArrivals int
		topupCounts   = map[string]int{}
		switchedNodes []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/user/topup":
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("读取请求体: %v", err)
				return
			}
			var payload map[string]string
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("解析请求体: %v", err)
				return
			}
			mu.Lock()
			topupCounts[payload["key"]]++
			limited := currentNode == "node-a"
			if limited {
				firstArrivals++
				if firstArrivals == workerCount {
					close(firstWave)
				}
			}
			mu.Unlock()
			if limited {
				select {
				case <-firstWave:
				case <-time.After(time.Second):
					t.Error("并行 worker 未同时进入首轮限流请求")
				}
				writer.WriteHeader(http.StatusTooManyRequests)
				_, _ = writer.Write([]byte(`{"success":false,"message":"请求过于频繁"}`))
				return
			}
			_, _ = writer.Write([]byte(`{"success":false,"message":"兑换码无效"}`))
		case strings.HasPrefix(request.URL.Path, "/proxies/") && strings.HasSuffix(request.URL.Path, "/delay"):
			_, _ = writer.Write([]byte(`{"delay":20}`))
		case request.URL.Path == "/group/test-group/delay":
			_, _ = writer.Write([]byte(`{"node-a":30,"node-b":10,"node-c":20}`))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodGet:
			mu.Lock()
			node := currentNode
			mu.Unlock()
			_, _ = writer.Write([]byte(fmt.Sprintf(`{"name":"test-group","type":"Selector","now":%q,"all":["node-a","node-b","node-c"]}`, node)))
		case request.URL.Path == "/proxies/test-group" && request.Method == http.MethodPut:
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("读取节点切换请求: %v", err)
				return
			}
			var payload map[string]string
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("解析节点切换请求: %v", err)
				return
			}
			mu.Lock()
			currentNode = payload["name"]
			switchedNodes = append(switchedNodes, currentNode)
			mu.Unlock()
			writer.WriteHeader(http.StatusNoContent)
		case request.URL.Path == "/proxies":
			_, _ = writer.Write([]byte(`{"proxies":{"node-a":{"type":"Trojan"},"node-b":{"type":"Trojan"},"node-c":{"type":"Trojan"}}}`))
		case request.URL.Path == "/ip":
			mu.Lock()
			node := currentNode
			mu.Unlock()
			if node == "node-c" {
				_, _ = writer.Write([]byte("2.2.2.2"))
				return
			}
			_, _ = writer.Write([]byte("1.1.1.1"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:                  t.TempDir(),
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		RequestInterval:          time.Millisecond,
		MihomoControllerURL:      server.URL,
		MihomoSelectorGroup:      "test-group",
		MihomoDelayURL:           server.URL + "/status",
		ExitIPURL:                server.URL + "/ip",
		MihomoHTTPClient:         server.Client(),
		MihomoSwitchPollInterval: time.Millisecond,
		MihomoSwitchPollAttempts: 2,
	})
	inputs := make([]NewAPIRedeemAccountInput, workerCount)
	for index := range inputs {
		inputs[index] = NewAPIRedeemAccountInput{UserID: fmt.Sprintf("limited-%d", index), AccessKey: fmt.Sprintf("key-%d", index)}
	}
	accounts, err := service.ImportAccounts(context.Background(), inputs)
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "limited.txt", []byte("limited-code-1\nlimited-code-2\nlimited-code-3\n"))
	require.NoError(t, err)
	accountIDs := make([]string, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	run, err := service.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{FileIDs: []string{file.ID}, AccountIDs: accountIDs})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, service, run.ID)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, 1, completed.SwitchCount)
	require.Equal(t, workerCount*2, completed.Attempts)
	require.Equal(t, workerCount, completed.CompletedPairs)
	mu.Lock()
	require.Equal(t, []string{"node-b", "node-c"}, switchedNodes)
	for _, count := range topupCounts {
		require.Equal(t, 2, count)
	}
	mu.Unlock()
}

func TestNewAPIRedeemRunNetworkSharesSwitchFailureAcrossWaiters(t *testing.T) {
	t.Parallel()
	const waiterCount = 8
	var (
		mu                  sync.Mutex
		groupDelayCalls     int
		firstDelayStarted   = make(chan struct{})
		releaseFirstFailure = make(chan struct{})
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/proxies/test-group":
			_, _ = writer.Write([]byte(`{"name":"test-group","type":"Selector","now":"node-a","all":["node-a","node-b"]}`))
		case "/proxies":
			_, _ = writer.Write([]byte(`{"proxies":{"node-a":{"type":"Trojan"},"node-b":{"type":"Trojan"}}}`))
		case "/group/test-group/delay":
			mu.Lock()
			groupDelayCalls++
			call := groupDelayCalls
			if call == 1 {
				close(firstDelayStarted)
			}
			mu.Unlock()
			if call == 1 {
				<-releaseFirstFailure
			}
			http.Error(writer, "delay unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:             t.TempDir(),
		MihomoControllerURL: server.URL,
		MihomoSelectorGroup: "test-group",
		MihomoHTTPClient:    server.Client(),
	})
	network := newNewAPIRedeemRunNetwork(service, "shared-switch-failure", waiterCount)
	network.state.accept(newAPIRedeemNetworkIdentity{Node: "node-a", ExitIP: "1.1.1.1"})

	start := make(chan struct{})
	errors := make(chan error, waiterCount)
	for range waiterCount {
		go func() {
			<-start
			_, err := network.switchExit(context.Background(), 0)
			errors <- err
		}()
	}
	close(start)
	<-firstDelayStarted
	time.Sleep(20 * time.Millisecond)
	close(releaseFirstFailure)
	for range waiterCount {
		require.ErrorContains(t, <-errors, "HTTP 503")
	}
	mu.Lock()
	require.Equal(t, 1, groupDelayCalls)
	mu.Unlock()
}

func TestNewAPIRedeemServiceInterruptsPersistedActiveRunAfterRestart(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/user/topup", request.URL.Path)
		_, _ = writer.Write([]byte(`{"success":true,"message":"充值成功"}`))
	}))
	defer server.Close()

	rootDir := t.TempDir()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:         rootDir,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
		RequestInterval: time.Millisecond,
	})
	accounts, err := service.ImportAccounts(context.Background(), []NewAPIRedeemAccountInput{{UserID: "14766", AccessKey: "access-key"}})
	require.NoError(t, err)
	file, err := service.SaveVoucherContent(context.Background(), "restart.txt", []byte("resume-code\n"))
	require.NoError(t, err)

	service.mu.Lock()
	service.state.Runs = append(service.state.Runs, NewAPIRedeemRun{
		ID:        "interrupted-by-restart",
		Status:    "running",
		StartedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		Message:   "正在按文件和账号顺序兑换",
		Logs:      []NewAPIRedeemSuccessLog{},
	})
	require.NoError(t, service.saveStateLocked())
	service.mu.Unlock()

	restarted := NewNewAPIRedeemService(NewAPIRedeemOptions{
		RootDir:         rootDir,
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
		RequestInterval: time.Millisecond,
	})
	overview, err := restarted.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, overview.Runs, 1)
	require.Equal(t, "interrupted", overview.Runs[0].Status)
	require.NotEmpty(t, overview.Runs[0].FinishedAt)
	require.Equal(t, "服务重启，兑换任务已中断", overview.Runs[0].Message)

	run, err := restarted.StartRedemption(context.Background(), NewAPIRedeemStartRunRequest{
		FileIDs:    []string{file.ID},
		AccountIDs: []string{accounts[0].ID},
	})
	require.NoError(t, err)
	completed := waitForNewAPIRedeemRun(t, restarted, run.ID)
	require.Equal(t, "completed", completed.Status)
}

func TestNewAPIRedeemServiceBoundsPersistedLogsWithoutLosingProgress(t *testing.T) {
	t.Parallel()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir()})
	service.mu.Lock()
	service.initialized = true
	service.state = newAPIRedeemState{
		Version:             1,
		Accounts:            []newAPIRedeemStoredAccount{},
		Files:               []newAPIRedeemStoredFile{},
		Redemptions:         []newAPIRedeemRedemption{},
		Runs:                []NewAPIRedeemRun{{ID: "bounded-logs", Status: "running", Logs: []NewAPIRedeemRequestLog{}}},
		PendingCodeRemovals: map[string][]string{},
	}
	require.NoError(t, os.MkdirAll(service.rootDir, 0o700))
	require.NoError(t, service.saveStateLocked())
	service.mu.Unlock()

	file := newAPIRedeemStoredFile{NewAPIRedeemVoucherFile: NewAPIRedeemVoucherFile{ID: "file-1", Name: "codes.txt"}}
	account := newAPIRedeemStoredAccount{ID: "account-1", UserID: "10001"}
	resultCount := newAPIRedeemMaxStoredLogs + newAPIRedeemCheckpointInterval
	for index := 0; index < resultCount; index++ {
		err := service.recordNewAPIRedeemRequest(
			"bounded-logs", file, account, fmt.Sprintf("code-%03d", index), "rejected", "rejected", http.StatusBadRequest, false,
			newAPIRedeemNetworkIdentity{}, nil, "已验证 codes.txt",
		)
		require.NoError(t, err)
	}
	service.finishNewAPIRedeemRun("bounded-logs", "completed", "兑换任务已完成")

	run, err := service.GetRun(context.Background(), "bounded-logs")
	require.NoError(t, err)
	require.Equal(t, resultCount, run.Attempts)
	require.Equal(t, resultCount, run.CompletedPairs)
	require.Len(t, run.Logs, newAPIRedeemMaxStoredLogs)
	require.Equal(t, fmt.Sprintf("code-%03d", newAPIRedeemCheckpointInterval), run.Logs[0].Code)
}

func TestNewAPIRedeemServiceKeepsLatestLogsAcrossAllRuns(t *testing.T) {
	t.Parallel()
	rootDir := t.TempDir()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	oldLogs := make([]NewAPIRedeemRequestLog, 60)
	newLogs := make([]NewAPIRedeemRequestLog, 80)
	for index := range oldLogs {
		oldLogs[index].Code = fmt.Sprintf("old-%03d", index)
	}
	for index := range newLogs {
		newLogs[index].Code = fmt.Sprintf("new-%03d", index)
	}
	service.mu.Lock()
	service.initialized = true
	service.state = newAPIRedeemState{
		Version:             1,
		Accounts:            []newAPIRedeemStoredAccount{},
		Files:               []newAPIRedeemStoredFile{},
		Redemptions:         []newAPIRedeemRedemption{},
		Runs:                []NewAPIRedeemRun{{ID: "old-run", Status: "completed", Logs: oldLogs}, {ID: "new-run", Status: "running", Logs: newLogs}},
		PendingCodeRemovals: map[string][]string{},
	}
	require.NoError(t, os.MkdirAll(service.rootDir, 0o700))
	require.NoError(t, service.saveStateLocked())
	service.mu.Unlock()

	restarted := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	overview, err := restarted.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, overview.Runs, 2)
	logsByRun := map[string][]NewAPIRedeemRequestLog{}
	for _, run := range overview.Runs {
		logsByRun[run.ID] = run.Logs
	}
	require.Len(t, logsByRun["old-run"], 20)
	require.Equal(t, "old-040", logsByRun["old-run"][0].Code)
	require.Len(t, logsByRun["new-run"], 80)

	file := newAPIRedeemStoredFile{NewAPIRedeemVoucherFile: NewAPIRedeemVoucherFile{ID: "file-1", Name: "codes.txt"}}
	account := newAPIRedeemStoredAccount{ID: "account-1", UserID: "10001"}
	for index := 0; index < newAPIRedeemCheckpointInterval; index++ {
		err = restarted.recordNewAPIRedeemRequest(
			"new-run", file, account, fmt.Sprintf("live-%03d", index), "rejected", "rejected", http.StatusBadRequest, false,
			newAPIRedeemNetworkIdentity{}, nil, "已验证 codes.txt",
		)
		require.NoError(t, err)
	}
	oldRun, err := restarted.GetRun(context.Background(), "old-run")
	require.NoError(t, err)
	require.Empty(t, oldRun.Logs)
	newRun, err := restarted.GetRun(context.Background(), "new-run")
	require.NoError(t, err)
	require.Len(t, newRun.Logs, newAPIRedeemMaxStoredLogs)
	require.Equal(t, "new-005", newRun.Logs[0].Code)
	require.Equal(t, "live-024", newRun.Logs[len(newRun.Logs)-1].Code)
}

func TestNewAPIRedeemServiceRecoversPendingCodeRemovalsAfterRestart(t *testing.T) {
	t.Parallel()
	rootDir := t.TempDir()
	service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	file, err := service.SaveVoucherContent(context.Background(), "codes.txt", []byte("code-a\ncode-b\ncode-c\n"))
	require.NoError(t, err)

	service.mu.Lock()
	index := findNewAPIRedeemFileByID(service.state.Files, file.ID)
	require.GreaterOrEqual(t, index, 0)
	service.state.Files[index].CodeCount = 1
	service.state.PendingCodeRemovals[file.ID] = []string{"code-a", "code-b"}
	require.NoError(t, service.saveStateLocked())
	storedName := service.state.Files[index].StoredName
	service.mu.Unlock()

	restarted := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
	overview, err := restarted.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, overview.Files, 1)
	require.Equal(t, 1, overview.Files[0].CodeCount)
	content, err := os.ReadFile(filepath.Join(rootDir, newAPIRedeemUploadsDirName, storedName))
	require.NoError(t, err)
	require.Equal(t, []string{"code-c"}, parseNewAPIRedeemCodes(content))
	restarted.mu.Lock()
	require.Empty(t, restarted.state.PendingCodeRemovals)
	restarted.mu.Unlock()
}

func BenchmarkNewAPIRedeemPersistence10000Codes(b *testing.B) {
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		rootDir := b.TempDir()
		service := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: rootDir})
		codes := make([]string, 10_000)
		for index := range codes {
			codes[index] = fmt.Sprintf("voucher-%05d", index)
		}
		file, err := service.SaveVoucherContent(context.Background(), "10k.txt", []byte(strings.Join(codes, "\n")))
		if err != nil {
			b.Fatal(err)
		}
		service.mu.Lock()
		storedIndex := findNewAPIRedeemFileByID(service.state.Files, file.ID)
		storedFile := service.state.Files[storedIndex]
		service.state.Runs = append(service.state.Runs, NewAPIRedeemRun{ID: "10k-run", Status: "running", TotalPairs: len(codes), Logs: []NewAPIRedeemRequestLog{}})
		if err := service.saveStateLocked(); err != nil {
			service.mu.Unlock()
			b.Fatal(err)
		}
		service.mu.Unlock()
		account := newAPIRedeemStoredAccount{ID: "account-1", UserID: "10001"}

		b.StartTimer()
		for _, code := range codes {
			_, pendingCount, err := service.stageNewAPIRedeemCodeRemoval(file.ID, code)
			if err != nil {
				b.Fatal(err)
			}
			if err := service.recordNewAPIRedeemRequest(
				"10k-run", storedFile, account, code, "invalid_code", "invalid", http.StatusBadRequest, true,
				newAPIRedeemNetworkIdentity{}, nil, "已验证 10k.txt",
			); err != nil {
				b.Fatal(err)
			}
			if pendingCount >= newAPIRedeemFileCompactBatch {
				if err := service.flushNewAPIRedeemPendingRemovals(file.ID); err != nil {
					b.Fatal(err)
				}
			}
		}
		if err := service.flushAllNewAPIRedeemPendingRemovals(); err != nil {
			b.Fatal(err)
		}
		service.finishNewAPIRedeemRun("10k-run", "completed", "兑换任务已完成")
		b.StopTimer()

		run, err := service.GetRun(context.Background(), "10k-run")
		if err != nil || run.CompletedPairs != len(codes) || len(run.Logs) != newAPIRedeemMaxStoredLogs {
			b.Fatalf("unexpected run state: err=%v completed=%d logs=%d", err, run.CompletedPairs, len(run.Logs))
		}
		stateInfo, err := os.Stat(service.statePath)
		if err != nil || stateInfo.Size() > 2*1024*1024 {
			b.Fatalf("state file is not bounded: err=%v size=%d", err, stateInfo.Size())
		}
		content, err := os.ReadFile(filepath.Join(service.uploadsDir, storedFile.StoredName))
		if err != nil || len(parseNewAPIRedeemCodes(content)) != 0 {
			b.Fatalf("voucher file was not compacted: err=%v", err)
		}
	}
}

func waitForNewAPIRedeemRun(t *testing.T, service *NewAPIRedeemService, runID string) NewAPIRedeemRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, err := service.GetRun(context.Background(), runID)
		require.NoError(t, err)
		if run.Status == "completed" || run.Status == "cancelled" || run.Status == "failed" {
			return run
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("兑换任务未在预期时间内完成")
	return NewAPIRedeemRun{}
}

func TestNewAPIRedeemOverviewAttachesSub2APIKeyReferences(t *testing.T) {
	t.Parallel()
	repo := &newAPICheckinAccountRepoStub{accounts: []Account{
		{ID: 11, Name: "已引用账号", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.www.cun.ai/v1", "api_keys": []any{"sk-abcd1234wxyz"}}},
		{ID: 12, Name: "可关联账号", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://www.cun.ai", "api_keys": []any{"sk-other"}}},
		{ID: 13, Name: "其他平台账号", Platform: PlatformOpenAI, Type: "oauth", Credentials: map[string]any{"base_url": "https://www.cun.ai"}},
	}}
	svc := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: "https://www.cun.ai", AccountRepository: repo})
	svc.mu.Lock()
	svc.initialized = true
	svc.state = newAPIRedeemState{Version: 1, Accounts: []newAPIRedeemStoredAccount{{ID: "redeem-1", UserID: "10001", AccessKey: "access", Snapshot: NewAPIRedeemAccount{APIKeys: []NewAPIRedeemAPIKey{{ID: 7, Name: "primary", MaskedKey: "sk-abcd****wxyz"}}}}}}
	svc.mu.Unlock()

	overview, err := svc.Overview(context.Background())
	require.NoError(t, err)
	require.Len(t, overview.Accounts, 1)
	require.Len(t, overview.Accounts[0].APIKeys[0].ReferencedAccounts, 1)
	require.Equal(t, int64(11), overview.Accounts[0].APIKeys[0].ReferencedAccounts[0].ID)
	require.Len(t, overview.Accounts[0].APIKeys[0].TargetAccounts, 2)
	require.True(t, overview.Accounts[0].APIKeys[0].TargetAccounts[0].Referenced)
	require.Equal(t, "可关联账号", overview.Accounts[0].APIKeys[0].TargetAccounts[1].Name)
}

func TestNewAPIRedeemLinkAPIKeyPreservesAndReplacesCredentials(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer source-access", request.Header.Get("Authorization"))
		_, _ = writer.Write([]byte(`{"success":true,"data":{"key":"sk-linked-secret"}}`))
	}))
	defer server.Close()
	repo := &newAPICheckinAccountRepoStub{accounts: []Account{{
		ID: 21, Name: "主账号", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": server.URL, "api_keys": []any{"sk-existing"}, "model_mapping": map[string]any{"a": "b"}, CredentialAPIKeysDisabled: map[string]any{"stale": map[string]any{}}},
	}}}
	svc := NewNewAPIRedeemService(NewAPIRedeemOptions{RootDir: t.TempDir(), BaseURL: server.URL, HTTPClient: server.Client(), AccountRepository: repo})
	svc.mu.Lock()
	svc.initialized = true
	svc.state = newAPIRedeemState{Version: 1, Accounts: []newAPIRedeemStoredAccount{{ID: "redeem-1", UserID: "10001", AccessKey: "source-access", BrowserProfileID: newAPIRedeemBrowserProfiles[0].ID}}}
	svc.mu.Unlock()

	result, err := svc.LinkAPIKeyToAccount(context.Background(), "redeem-1", 7, 21, "append")
	require.NoError(t, err)
	require.Equal(t, 2, result.KeyCount)
	require.Equal(t, []string{"sk-existing", "sk-linked-secret"}, repo.updated.GetAPIKeys())
	require.NotNil(t, repo.updated.Credentials["model_mapping"])

	result, err = svc.LinkAPIKeyToAccount(context.Background(), "redeem-1", 7, 21, "replace")
	require.NoError(t, err)
	require.Equal(t, 1, result.KeyCount)
	require.Equal(t, []string{"sk-linked-secret"}, repo.updated.GetAPIKeys())
	_, disabledPresent := repo.updated.Credentials[CredentialAPIKeysDisabled]
	require.False(t, disabledPresent)
}

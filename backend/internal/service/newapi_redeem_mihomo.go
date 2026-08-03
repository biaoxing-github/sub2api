package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	newAPIRedeemDefaultMihomoControllerURL = "http://host.docker.internal:9097"
	newAPIRedeemDefaultMihomoSelector      = "🚀 节点选择"
	newAPIRedeemDefaultMihomoDelayURL      = "https://www.cun.ai/api/status"
	newAPIRedeemDefaultExitIPURL           = "https://api.ipify.org"
	// newAPIRedeemMihomoGroupDelayTimeoutMillis 是 Controller 批量测速的单节点超时。
	newAPIRedeemMihomoGroupDelayTimeoutMillis = 5000
	// newAPIRedeemMihomoSwitchTimeout 限制一次限流切换占用全局网络闸门的最长时间。
	newAPIRedeemMihomoSwitchTimeout = 20 * time.Second
)

// newAPIRedeemNetworkIdentity 表示一次兑换请求实际使用的节点和出口 IP。
type newAPIRedeemNetworkIdentity struct {
	Node   string
	ExitIP string
}

// newAPIRedeemNetworkSwitch 记录一次限流后的节点和出口 IP 变化。
type newAPIRedeemNetworkSwitch struct {
	FromNode string
	FromIP   string
	ToNode   string
	ToIP     string
	Message  string
}

// newAPIRedeemNetworkState 只在单次兑换任务内使用，保证节点名和出口 IP 都不重复。
type newAPIRedeemNetworkState struct {
	Current     newAPIRedeemNetworkIdentity
	UsedNodes   map[string]struct{}
	UsedIPs     map[string]struct{}
	UsedRegions map[string]struct{}
}

func newNewAPIRedeemNetworkState() *newAPIRedeemNetworkState {
	return &newAPIRedeemNetworkState{
		UsedNodes:   map[string]struct{}{},
		UsedIPs:     map[string]struct{}{},
		UsedRegions: map[string]struct{}{},
	}
}

func (state *newAPIRedeemNetworkState) accept(identity newAPIRedeemNetworkIdentity) {
	state.Current = identity
	if identity.Node != "" {
		state.UsedNodes[identity.Node] = struct{}{}
	}
	if identity.ExitIP != "" {
		state.UsedIPs[identity.ExitIP] = struct{}{}
	}
	if region := newAPIRedeemMihomoNodeRegion(identity.Node); region != "" {
		if state.UsedRegions == nil {
			state.UsedRegions = map[string]struct{}{}
		}
		state.UsedRegions[region] = struct{}{}
	}
}

// newAPIRedeemMihomoNodeRegion 提取节点名称的地区前缀，用于在延迟相近时轮换不同地区。
func newAPIRedeemMihomoNodeRegion(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// newAPIRedeemMihomoClient 通过 Mihomo 官方 REST API 读取并切换 Clash Verge 节点。
type newAPIRedeemMihomoClient struct {
	controllerURL string
	secret        string
	selectorGroup string
	delayURL      string
	exitIPURL     string
	httpClient    *http.Client
	pollInterval  time.Duration
	pollAttempts  int
}

type newAPIRedeemMihomoDelayedNode struct {
	name   string
	region string
	delay  int64
}

// prioritizeNewAPIRedeemMihomoRegions 将尚未尝试过的地区排在前面，避免 20 秒切换窗口只消耗同一地区。
func prioritizeNewAPIRedeemMihomoRegions(nodes []newAPIRedeemMihomoDelayedNode, usedRegions map[string]struct{}) []newAPIRedeemMihomoDelayedNode {
	remaining := append([]newAPIRedeemMihomoDelayedNode(nil), nodes...)
	ordered := make([]newAPIRedeemMihomoDelayedNode, 0, len(nodes))
	roundRegions := make(map[string]struct{}, len(usedRegions))
	for region := range usedRegions {
		roundRegions[region] = struct{}{}
	}
	for len(remaining) > 0 {
		selected := -1
		for index, node := range remaining {
			if _, used := roundRegions[node.region]; !used {
				selected = index
				break
			}
		}
		if selected < 0 {
			for region := range roundRegions {
				delete(roundRegions, region)
			}
			selected = 0
		}
		node := remaining[selected]
		ordered = append(ordered, node)
		remaining = append(remaining[:selected], remaining[selected+1:]...)
		roundRegions[node.region] = struct{}{}
	}
	return ordered
}

func newNewAPIRedeemMihomoClient(options NewAPIRedeemOptions, fallbackClient *http.Client) *newAPIRedeemMihomoClient {
	controllerURL := strings.TrimSpace(options.MihomoControllerURL)
	if controllerURL == "" {
		return nil
	}
	selectorGroup := firstNewAPIRedeemText(options.MihomoSelectorGroup, newAPIRedeemDefaultMihomoSelector)
	delayURL := firstNewAPIRedeemText(options.MihomoDelayURL, newAPIRedeemDefaultMihomoDelayURL)
	exitIPURL := firstNewAPIRedeemText(options.ExitIPURL, newAPIRedeemDefaultExitIPURL)
	client := options.MihomoHTTPClient
	if client == nil {
		client = fallbackClient
	}
	pollInterval := options.MihomoSwitchPollInterval
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	pollAttempts := options.MihomoSwitchPollAttempts
	if pollAttempts <= 0 {
		pollAttempts = 10
	}
	return &newAPIRedeemMihomoClient{
		controllerURL: strings.TrimRight(controllerURL, "/"),
		secret:        strings.TrimSpace(options.MihomoControllerSecret),
		selectorGroup: selectorGroup,
		delayURL:      delayURL,
		exitIPURL:     exitIPURL,
		httpClient:    client,
		pollInterval:  pollInterval,
		pollAttempts:  pollAttempts,
	}
}

// inspect 返回当前选择器节点和当前容器请求看到的出口 IP。
func (client *newAPIRedeemMihomoClient) inspect(ctx context.Context) (newAPIRedeemNetworkIdentity, error) {
	group, err := client.proxyGroup(ctx)
	if err != nil {
		return newAPIRedeemNetworkIdentity{}, err
	}
	node := newAPIRedeemString(group["now"])
	if node == "" {
		return newAPIRedeemNetworkIdentity{}, fmt.Errorf("Mihomo 选择器未返回当前节点")
	}
	exitIP, err := client.exitIP(ctx)
	if err != nil {
		return newAPIRedeemNetworkIdentity{}, err
	}
	return newAPIRedeemNetworkIdentity{Node: node, ExitIP: exitIP}, nil
}

// switchExit 通过 Controller 批量测速排序未使用节点，只有检测到全新出口 IP 时才接受切换结果。
func (client *newAPIRedeemMihomoClient) switchExit(ctx context.Context, state *newAPIRedeemNetworkState) (newAPIRedeemNetworkSwitch, error) {
	switchCtx, cancel := context.WithTimeout(ctx, newAPIRedeemMihomoSwitchTimeout)
	defer cancel()
	if state.Current.Node == "" || state.Current.ExitIP == "" {
		identity, err := client.inspect(switchCtx)
		if err != nil {
			return newAPIRedeemNetworkSwitch{}, err
		}
		state.accept(identity)
	}
	group, err := client.proxyGroup(switchCtx)
	if err != nil {
		return newAPIRedeemNetworkSwitch{}, err
	}
	proxies, err := client.proxies(switchCtx)
	if err != nil {
		return newAPIRedeemNetworkSwitch{}, err
	}
	delays, err := client.groupDelays(switchCtx)
	if err != nil {
		return newAPIRedeemNetworkSwitch{}, err
	}
	candidates := make([]string, 0)
	for _, raw := range newAPIRedeemSlice(group["all"]) {
		name := newAPIRedeemString(raw)
		if name == "" || name == state.Current.Node {
			continue
		}
		if _, used := state.UsedNodes[name]; used {
			continue
		}
		proxy := newAPIRedeemMap(proxies[name])
		if !isNewAPIRedeemConcreteMihomoNode(newAPIRedeemString(proxy["type"])) {
			continue
		}
		delay := newAPIRedeemInt64(delays[name])
		if delay <= 0 {
			continue
		}
		candidates = append(candidates, name)
	}
	if len(candidates) == 0 {
		return newAPIRedeemNetworkSwitch{}, fmt.Errorf("没有剩余的未使用 Mihomo 节点")
	}

	available := make([]newAPIRedeemMihomoDelayedNode, 0, len(candidates))
	for _, candidate := range candidates {
		delay := newAPIRedeemInt64(delays[candidate])
		available = append(available, newAPIRedeemMihomoDelayedNode{
			name:   candidate,
			region: newAPIRedeemMihomoNodeRegion(candidate),
			delay:  delay,
		})
	}
	sort.SliceStable(available, func(i, j int) bool { return available[i].delay < available[j].delay })
	if len(available) == 0 {
		return newAPIRedeemNetworkSwitch{}, fmt.Errorf("未找到延迟测试可用的 Mihomo 节点")
	}

	if state.UsedRegions == nil {
		state.UsedRegions = map[string]struct{}{}
	}
	available = prioritizeNewAPIRedeemMihomoRegions(available, state.UsedRegions)
	from := state.Current
	for _, candidate := range available {
		if err := switchCtx.Err(); err != nil {
			return newAPIRedeemNetworkSwitch{}, err
		}
		state.UsedNodes[candidate.name] = struct{}{}
		if candidate.region != "" {
			state.UsedRegions[candidate.region] = struct{}{}
		}
		if err := client.selectNode(switchCtx, candidate.name); err != nil {
			continue
		}
		for attempt := 0; attempt < client.pollAttempts; attempt++ {
			if attempt > 0 {
				select {
				case <-switchCtx.Done():
					return newAPIRedeemNetworkSwitch{}, switchCtx.Err()
				case <-time.After(client.pollInterval):
				}
			}
			exitIP, exitErr := client.exitIP(switchCtx)
			if exitErr != nil || exitIP == "" {
				continue
			}
			if _, used := state.UsedIPs[exitIP]; used {
				continue
			}
			identity := newAPIRedeemNetworkIdentity{Node: candidate.name, ExitIP: exitIP}
			state.accept(identity)
			return newAPIRedeemNetworkSwitch{
				FromNode: from.Node,
				FromIP:   from.ExitIP,
				ToNode:   identity.Node,
				ToIP:     identity.ExitIP,
				Message:  fmt.Sprintf("已切换到 %s，出口 IP %s", identity.Node, identity.ExitIP),
			}, nil
		}
	}
	return newAPIRedeemNetworkSwitch{}, fmt.Errorf("所有可用节点的出口 IP 均已使用或未发生变化")
}

func (client *newAPIRedeemMihomoClient) proxyGroup(ctx context.Context) (map[string]any, error) {
	return client.controllerJSON(ctx, http.MethodGet, "/proxies/"+url.PathEscape(client.selectorGroup), nil)
}

func (client *newAPIRedeemMihomoClient) proxies(ctx context.Context) (map[string]any, error) {
	payload, err := client.controllerJSON(ctx, http.MethodGet, "/proxies", nil)
	if err != nil {
		return nil, err
	}
	return newAPIRedeemMap(payload["proxies"]), nil
}

// groupDelays 使用 Mihomo 官方分组测速接口一次性获取所有节点延迟，避免逐节点串行请求拖住全局切换锁。
func (client *newAPIRedeemMihomoClient) groupDelays(ctx context.Context) (map[string]any, error) {
	path := fmt.Sprintf(
		"/group/%s/delay?url=%s&timeout=%d",
		url.PathEscape(client.selectorGroup),
		url.QueryEscape(client.delayURL),
		newAPIRedeemMihomoGroupDelayTimeoutMillis,
	)
	return client.controllerJSON(ctx, http.MethodGet, path, nil)
}

func (client *newAPIRedeemMihomoClient) selectNode(ctx context.Context, node string) error {
	_, err := client.controllerJSON(ctx, http.MethodPut, "/proxies/"+url.PathEscape(client.selectorGroup), map[string]any{"name": node})
	return err
}

func (client *newAPIRedeemMihomoClient) controllerJSON(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("编码 Mihomo 请求: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, client.controllerURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("创建 Mihomo 请求: %w", err)
	}
	req.Close = true
	if client.secret != "" {
		req.Header.Set("Authorization", "Bearer "+client.secret)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Mihomo Controller 失败: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取 Mihomo 响应: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Mihomo Controller 返回 HTTP %d", response.StatusCode)
	}
	if len(content) == 0 {
		return map[string]any{}, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("解析 Mihomo 响应: %w", err)
	}
	return payload, nil
}

func (client *newAPIRedeemMihomoClient) exitIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.exitIPURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建出口 IP 请求: %w", err)
	}
	req.Close = true
	req.Header.Set("Accept", "application/json, text/plain")
	response, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("读取出口 IP 失败: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("读取出口 IP 响应: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("出口 IP 服务返回 HTTP %d", response.StatusCode)
	}
	text := strings.TrimSpace(string(content))
	if strings.HasPrefix(text, "{") {
		payload := map[string]any{}
		if err := json.Unmarshal(content, &payload); err == nil {
			text = firstNewAPIRedeemText(newAPIRedeemString(payload["ip"]), newAPIRedeemString(payload["origin"]), newAPIRedeemString(payload["address"]))
		}
	}
	ip := net.ParseIP(strings.TrimSpace(text))
	if ip == nil {
		return "", fmt.Errorf("出口 IP 服务返回无效地址")
	}
	return ip.String(), nil
}

func isNewAPIRedeemConcreteMihomoNode(proxyType string) bool {
	switch strings.ToLower(strings.TrimSpace(proxyType)) {
	case "", "selector", "urltest", "fallback", "loadbalance", "relay", "compatible", "direct", "reject", "pass":
		return false
	default:
		return true
	}
}

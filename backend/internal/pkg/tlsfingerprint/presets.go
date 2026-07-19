package tlsfingerprint

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	utls "github.com/refraction-networking/utls"
)

// ClientHelloPreset 标识由 uTLS 维护的固定客户端 ClientHello 模板。
type ClientHelloPreset string

const (
	ClientHelloPresetChrome100    ClientHelloPreset = "chrome_100"
	ClientHelloPresetIOS121       ClientHelloPreset = "ios_12_1"
	ClientHelloPresetFirefox105   ClientHelloPreset = "firefox_105"
	ClientHelloPresetFirefox120   ClientHelloPreset = "firefox_120"
	ClientHelloPresetSafari160    ClientHelloPreset = "safari_16_0"
	ClientHelloPresetIOS14        ClientHelloPreset = "ios_14"
	ClientHelloPresetAndroid11    ClientHelloPreset = "android_11_okhttp"
	ClientHelloPresetEdge85       ClientHelloPreset = "edge_85"
	ClientHelloPreset360Browser75 ClientHelloPreset = "360_browser_7_5"
	ClientHelloPresetQQBrowser111 ClientHelloPreset = "qq_browser_11_1"
)

// buildClientHelloSpec 根据 Profile 选择固定预设或原有自定义字段模板。
func buildClientHelloSpec(profile *Profile) (*utls.ClientHelloSpec, error) {
	if profile == nil || profile.Preset == "" {
		return buildClientHelloSpecFromProfile(profile), nil
	}

	id, ok := clientHelloIDForPreset(profile.Preset)
	if !ok {
		return nil, fmt.Errorf("unsupported TLS ClientHello preset: %s", profile.Preset)
	}
	spec, err := utls.UTLSIdToSpec(id)
	if err != nil {
		return nil, fmt.Errorf("build TLS ClientHello preset %s: %w", profile.Preset, err)
	}
	forceHTTP1ClientHello(&spec)
	return &spec, nil
}

// clientHelloIDForPreset 把稳定的业务枚举映射到对应 uTLS 模板。
func clientHelloIDForPreset(preset ClientHelloPreset) (utls.ClientHelloID, bool) {
	switch preset {
	case ClientHelloPresetChrome100:
		return utls.HelloChrome_100, true
	case ClientHelloPresetIOS121:
		return utls.HelloIOS_12_1, true
	case ClientHelloPresetFirefox105:
		return utls.HelloFirefox_105, true
	case ClientHelloPresetFirefox120:
		return utls.HelloFirefox_120, true
	case ClientHelloPresetSafari160:
		return utls.HelloSafari_16_0, true
	case ClientHelloPresetIOS14:
		return utls.HelloIOS_14, true
	case ClientHelloPresetAndroid11:
		return utls.HelloAndroid_11_OkHttp, true
	case ClientHelloPresetEdge85:
		return utls.HelloEdge_85, true
	case ClientHelloPreset360Browser75:
		return utls.Hello360_7_5, true
	case ClientHelloPresetQQBrowser111:
		return utls.HelloQQ_11_1, true
	default:
		return utls.ClientHelloID{}, false
	}
}

// forceHTTP1ClientHello 保留预设的密码套件和扩展顺序，仅把应用层协议限制为 HTTP/1.1。
func forceHTTP1ClientHello(spec *utls.ClientHelloSpec) {
	if spec == nil {
		return
	}

	hasALPN := false
	extensions := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, extension := range spec.Extensions {
		switch typed := extension.(type) {
		case *utls.ALPNExtension:
			cloned := *typed
			cloned.AlpnProtocols = []string{"http/1.1"}
			extensions = append(extensions, &cloned)
			hasALPN = true
		case *utls.ApplicationSettingsExtension, *utls.ApplicationSettingsExtensionNew:
			// ALPS 只服务于 HTTP/2/HTTP/3，当前上游 WebSocket 握手必须使用 HTTP/1.1。
			continue
		default:
			extensions = append(extensions, extension)
		}
	}
	if !hasALPN {
		extensions = append(extensions, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
	}
	spec.Extensions = extensions
}

// ProfileCacheKey 返回完整 Profile 内容的稳定摘要，用于隔离底层连接池。
func ProfileCacheKey(profile *Profile) string {
	if profile == nil {
		return "none"
	}

	identity := struct {
		Name                string
		Preset              ClientHelloPreset
		CipherSuites        []uint16
		Curves              []uint16
		PointFormats        []uint16
		EnableGREASE        bool
		SignatureAlgorithms []uint16
		ALPNProtocols       []string
		SupportedVersions   []uint16
		KeyShareGroups      []uint16
		PSKModes            []uint16
		Extensions          []uint16
	}{
		Name:                profile.Name,
		Preset:              profile.Preset,
		CipherSuites:        profile.CipherSuites,
		Curves:              profile.Curves,
		PointFormats:        profile.PointFormats,
		EnableGREASE:        profile.EnableGREASE,
		SignatureAlgorithms: profile.SignatureAlgorithms,
		ALPNProtocols:       profile.ALPNProtocols,
		SupportedVersions:   profile.SupportedVersions,
		KeyShareGroups:      profile.KeyShareGroups,
		PSKModes:            profile.PSKModes,
		Extensions:          profile.Extensions,
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		panic(fmt.Sprintf("marshal TLS profile cache identity: %v", err))
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:])
}

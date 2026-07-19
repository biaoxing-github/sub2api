//go:build unit

package tlsfingerprint

import (
	"fmt"
	"net"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

// TestBuiltInClientHelloPresets 验证十套固定预设都可构造、互不相同且只协商 HTTP/1.1。
func TestBuiltInClientHelloPresets(t *testing.T) {
	profiles := []Profile{
		{Name: "Chrome 100", Preset: ClientHelloPresetChrome100},
		{Name: "iOS 12.1", Preset: ClientHelloPresetIOS121},
		{Name: "Firefox 105", Preset: ClientHelloPresetFirefox105},
		{Name: "Firefox 120", Preset: ClientHelloPresetFirefox120},
		{Name: "Safari 16.0", Preset: ClientHelloPresetSafari160},
		{Name: "iOS 14", Preset: ClientHelloPresetIOS14},
		{Name: "Android 11 OkHttp", Preset: ClientHelloPresetAndroid11},
		{Name: "Edge 85", Preset: ClientHelloPresetEdge85},
		{Name: "360 Browser 7.5", Preset: ClientHelloPreset360Browser75},
		{Name: "QQ Browser 11.1", Preset: ClientHelloPresetQQBrowser111},
	}
	require.Len(t, profiles, 10)

	seenSpecs := make(map[string]string, len(profiles))
	seenKeys := make(map[string]string, len(profiles))
	for i := range profiles {
		profile := profiles[i]
		t.Run(profile.Name, func(t *testing.T) {
			spec, err := buildClientHelloSpec(&profile)
			require.NoError(t, err)
			require.NotEmpty(t, spec.CipherSuites)
			require.NotEmpty(t, spec.Extensions)
			clientConn, serverConn := net.Pipe()
			t.Cleanup(func() {
				_ = clientConn.Close()
				_ = serverConn.Close()
			})
			tlsConn := utls.UClient(clientConn, &utls.Config{ServerName: "example.com"}, utls.HelloCustom)
			require.NoError(t, tlsConn.ApplyPreset(spec))

			for _, extension := range spec.Extensions {
				switch typed := extension.(type) {
				case *utls.ALPNExtension:
					require.Equal(t, []string{"http/1.1"}, typed.AlpnProtocols)
				case *utls.ApplicationSettingsExtension, *utls.ApplicationSettingsExtensionNew:
					t.Fatalf("预设 %s 不应保留 ALPS 扩展 %T", profile.Name, extension)
				}
			}

			specIdentity := clientHelloSpecTestIdentity(spec)
			if previous, exists := seenSpecs[specIdentity]; exists {
				t.Fatalf("预设 %s 与 %s 生成了相同 ClientHello", profile.Name, previous)
			}
			seenSpecs[specIdentity] = profile.Name

			cacheKey := ProfileCacheKey(&profile)
			if previous, exists := seenKeys[cacheKey]; exists {
				t.Fatalf("预设 %s 与 %s 生成了相同缓存键", profile.Name, previous)
			}
			seenKeys[cacheKey] = profile.Name
		})
	}
}

// TestBuildClientHelloSpecRejectsUnknownPreset 验证未知预设直接返回可诊断错误。
func TestBuildClientHelloSpecRejectsUnknownPreset(t *testing.T) {
	_, err := buildClientHelloSpec(&Profile{Name: "invalid", Preset: ClientHelloPreset("invalid")})
	require.ErrorContains(t, err, "unsupported TLS ClientHello preset")
}

// TestProfileCacheKeyUsesCompleteProfile 验证任一有效配置变化都会改变连接池摘要。
func TestProfileCacheKeyUsesCompleteProfile(t *testing.T) {
	base := &Profile{Name: "custom", CipherSuites: []uint16{0x1301}, ALPNProtocols: []string{"http/1.1"}}
	same := &Profile{Name: "custom", CipherSuites: []uint16{0x1301}, ALPNProtocols: []string{"http/1.1"}}
	changed := &Profile{Name: "custom", CipherSuites: []uint16{0x1302}, ALPNProtocols: []string{"http/1.1"}}

	require.Equal(t, ProfileCacheKey(base), ProfileCacheKey(same))
	require.NotEqual(t, ProfileCacheKey(base), ProfileCacheKey(changed))
	require.Equal(t, "none", ProfileCacheKey(nil))
}

// clientHelloSpecTestIdentity 生成仅供测试比较使用的完整 ClientHello 文本身份。
func clientHelloSpecTestIdentity(spec *utls.ClientHelloSpec) string {
	parts := []string{
		fmt.Sprintf("cipher=%v", spec.CipherSuites),
		fmt.Sprintf("compression=%v", spec.CompressionMethods),
		fmt.Sprintf("versions=%d:%d", spec.TLSVersMin, spec.TLSVersMax),
	}
	for _, extension := range spec.Extensions {
		parts = append(parts, fmt.Sprintf("%T:%#v", extension, extension))
	}
	return strings.Join(parts, "|")
}

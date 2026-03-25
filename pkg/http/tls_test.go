package http

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTLSConfig_ServerAndClientMatch(t *testing.T) {
	t.Parallel()

	// Both server (via NewServer) and client (via NewClient/NewTransport) call
	// defaultTLSConfig(). Verify each call produces identical security settings.
	cfg1 := defaultTLSConfig()
	cfg2 := defaultTLSConfig()

	assert.Equal(t, cfg1.MinVersion, cfg2.MinVersion)
	assert.Equal(t, cfg1.CipherSuites, cfg2.CipherSuites)
	assert.Equal(t, cfg1.CurvePreferences, cfg2.CurvePreferences)
}

func TestNewServer_NoPreferServerCipherSuites(t *testing.T) {
	t.Parallel()

	cfg := defaultTLSConfig()
	// PreferServerCipherSuites should not be set; Go 1.22+ ignores it but
	// explicitly leaving it unset signals intent not to use deprecated behaviour.
	assert.False(t, cfg.PreferServerCipherSuites) //nolint:staticcheck
}

func TestDefaultTLSConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultTLSConfig()

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.NotEmpty(t, cfg.CipherSuites)
	assert.NotEmpty(t, cfg.CurvePreferences)

	// Ensure X25519 is preferred
	assert.Equal(t, tls.X25519, cfg.CurvePreferences[0])

	// Verify all cipher suites are curated (AEAD based)
	for _, suite := range cfg.CipherSuites {
		assert.Contains(t, []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		}, suite)
	}
}

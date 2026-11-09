package config

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SMTP_ENABLED", "false")
	t.Setenv("STORAGE_DIR", t.TempDir())
}

func TestTrustedProxiesDefaultTrustsAll(t *testing.T) {
	setBaseEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/0"),
		netip.MustParsePrefix("::/0"),
	}
	if !slices.Equal(cfg.Server.TrustedProxies, want) {
		t.Errorf("TrustedProxies = %v, want %v", cfg.Server.TrustedProxies, want)
	}
}

func TestTrustedProxiesParsesIPsAndRanges(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, ::1,10.1.2.3/8")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}
	if !slices.Equal(cfg.Server.TrustedProxies, want) {
		t.Errorf("TrustedProxies = %v, want %v", cfg.Server.TrustedProxies, want)
	}
}

func TestTrustedProxiesNoneTrustsNone(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TRUSTED_PROXIES", "none")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Server.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want none", cfg.Server.TrustedProxies)
	}
}

func TestTrustedProxiesRejectsBadValue(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,not-an-ip")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Errorf("Load() error = %v, want TRUSTED_PROXIES error", err)
	}
}

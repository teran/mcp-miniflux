package config

import (
	"errors"
	"testing"
)

// CONTRACT — infrastructure/config loader (SPEC §6.4, §9 table; §3.1 token
// resolution).
//
// The developer must define the following in package `config` (file
// infrastructure/config/config.go). This test file is written FIRST (TDD, red
// state) and will not compile until the loader exists. The exact contract:
//
//	var ErrMissingAPIURL   = errors.New("...") // MINIFLUX_API_URL is REQUIRED (no default, both modes)
//	var ErrMissingAPIToken = errors.New("...") // MINIFLUX_API_TOKEN is REQUIRED in stdio mode only (secret)
//
//	// Load reads env (via envconfig) into a fresh domain.Config, applies
//	// domain.Config.SetDefaults, then runs mode-aware validation. `mode` is one
//	// of "http" or "stdio".
//	//
//	// Validation rules:
//	//   - MINIFLUX_API_URL absent/empty -> ErrMissingAPIURL in BOTH modes.
//	//   - MINIFLUX_API_TOKEN absent/empty -> ErrMissingAPIToken in stdio mode.
//	//   - MINIFLUX_API_TOKEN absent/empty in http mode -> NO error (token is
//	//     optional there; auth is provided by the inbound X-Auth-Token
//	//     pass-through / runtime fallback, SPEC §3.1).
//	//
//	// Any envconfig parse error is returned as-is.
//	func Load(mode string) (domain.Config, error)
//
// DESIGN DECISION (documented): the loader builds a zero-value domain.Config and
// passes it to envconfig.Process("", &cfg) using the envconfig struct tags on
// domain.Config (the tags are locked by domain/config_test.go), then calls
// cfg.SetDefaults() so explicitly-configured env values win over the documented
// defaults. Required-field validation is done AFTER defaults are applied, and is
// mode-aware: MINIFLUX_API_URL is required in both modes, while
// MINIFLUX_API_TOKEN is required only in stdio mode (never filled by
// SetDefaults — it has no default — so a missing/empty value is detected and
// reported via the sentinel error).

// setEnv sets every config env var to the value in v, or "" (empty) when absent
// from v. Setting an empty value is equivalent to clearing it for this loader,
// and it makes each test hermetic against any ambient environment.
func setEnv(t *testing.T, v map[string]string) {
	t.Helper()
	keys := []string{
		"MINIFLUX_API_URL",
		"MINIFLUX_API_TOKEN",
		"LISTEN_ADDR",
		"INTERNAL_ADDR",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"LOG_FILENAME",
	}
	for _, k := range keys {
		t.Setenv(k, v[k])
	}
}

// TestLoadPopulatesAllFields locks the happy path in BOTH modes: every env var
// maps onto its domain.Config field, and both required vars being present is
// valid for http and stdio alike. If any mapping is wrong the corresponding
// assertion fails (mutation guard).
func TestLoadPopulatesAllFields(t *testing.T) {
	for _, mode := range []string{"http", "stdio"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			setEnv(t, map[string]string{
				"MINIFLUX_API_URL":   "https://reader.example.com",
				"MINIFLUX_API_TOKEN": "tok-123",
				"LISTEN_ADDR":        ":9090",
				"INTERNAL_ADDR":      ":9091",
				"LOG_LEVEL":          "debug",
				"LOG_FORMAT":         "json",
				"LOG_FILENAME":       "/var/log/mcp.log",
			})

			cfg, err := Load(mode)
			if err != nil {
				t.Fatalf("Load(%q) returned error: %v", mode, err)
			}

			if cfg.MinifluxAPIURL != "https://reader.example.com" {
				t.Errorf("MinifluxAPIURL = %q, want %q", cfg.MinifluxAPIURL, "https://reader.example.com")
			}
			if cfg.MinifluxAPIToken != "tok-123" {
				t.Errorf("MinifluxAPIToken = %q, want %q", cfg.MinifluxAPIToken, "tok-123")
			}
			if cfg.ListenAddr != ":9090" {
				t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
			}
			if cfg.InternalAddr != ":9091" {
				t.Errorf("InternalAddr = %q, want %q", cfg.InternalAddr, ":9091")
			}
			if cfg.LogLevel != "debug" {
				t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
			}
			if cfg.LogFormat != "json" {
				t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
			}
			if cfg.LogFilename != "/var/log/mcp.log" {
				t.Errorf("LogFilename = %q, want %q", cfg.LogFilename, "/var/log/mcp.log")
			}
		})
	}
}

// TestLoadAppliesDefaults locks the default resolution (§9 table) in BOTH modes:
// when the optional env vars are absent, Load returns the documented defaults,
// while the required fields (API URL, API token) are taken from env and
// LogFilename stays empty (no default).
func TestLoadAppliesDefaults(t *testing.T) {
	for _, mode := range []string{"http", "stdio"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			setEnv(t, map[string]string{
				"MINIFLUX_API_URL":   "https://reader.example.com",
				"MINIFLUX_API_TOKEN": "tok-123",
			})

			cfg, err := Load(mode)
			if err != nil {
				t.Fatalf("Load(%q) returned error: %v", mode, err)
			}

			if cfg.ListenAddr != ":8080" {
				t.Errorf("ListenAddr = %q, want default %q", cfg.ListenAddr, ":8080")
			}
			if cfg.InternalAddr != ":8081" {
				t.Errorf("InternalAddr = %q, want default %q", cfg.InternalAddr, ":8081")
			}
			if cfg.LogLevel != "info" {
				t.Errorf("LogLevel = %q, want default %q", cfg.LogLevel, "info")
			}
			if cfg.LogFormat != "text" {
				t.Errorf("LogFormat = %q, want default %q", cfg.LogFormat, "text")
			}
			if cfg.LogFilename != "" {
				t.Errorf("LogFilename = %q, want empty (no default)", cfg.LogFilename)
			}
		})
	}
}

// TestLoadMissingAPIURL locks the REQUIRED contract: MINIFLUX_API_URL has no
// default and is required in BOTH modes, so an absent value must fail with
// ErrMissingAPIURL regardless of mode.
func TestLoadMissingAPIURL(t *testing.T) {
	for _, mode := range []string{"http", "stdio"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			setEnv(t, map[string]string{
				"MINIFLUX_API_TOKEN": "tok-123",
			})

			_, err := Load(mode)
			if err == nil {
				t.Fatalf("Load(%q) = nil error, want ErrMissingAPIURL", mode)
			}
			if !errors.Is(err, ErrMissingAPIURL) {
				t.Errorf("Load(%q) error = %v, want ErrMissingAPIURL", mode, err)
			}
		})
	}
}

// TestLoadExplicitEmptyAPIURL is the boundary of the required check: even when
// the variable is present but set to an empty string, it is treated as missing
// in both modes.
func TestLoadExplicitEmptyAPIURL(t *testing.T) {
	for _, mode := range []string{"http", "stdio"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			setEnv(t, map[string]string{
				"MINIFLUX_API_URL":   "",
				"MINIFLUX_API_TOKEN": "tok-123",
			})

			_, err := Load(mode)
			if err == nil {
				t.Fatalf("Load(%q) = nil error with empty MINIFLUX_API_URL, want ErrMissingAPIURL", mode)
			}
			if !errors.Is(err, ErrMissingAPIURL) {
				t.Errorf("Load(%q) error = %v, want ErrMissingAPIURL", mode, err)
			}
		})
	}
}

// TestLoadMissingAPITokenStdioRejected locks the REQUIRED contract for the
// secret token in stdio mode: MINIFLUX_API_TOKEN has no default, so in stdio an
// absent value must fail with ErrMissingAPIToken.
func TestLoadMissingAPITokenStdioRejected(t *testing.T) {
	setEnv(t, map[string]string{
		"MINIFLUX_API_URL": "https://reader.example.com",
	})

	_, err := Load("stdio")
	if err == nil {
		t.Fatal("Load(\"stdio\") = nil error, want ErrMissingAPIToken")
	}
	if !errors.Is(err, ErrMissingAPIToken) {
		t.Errorf("Load(\"stdio\") error = %v, want ErrMissingAPIToken", err)
	}
}

// TestLoadExplicitEmptyAPITokenStdioRejected is the boundary of the stdio token
// check: even when MINIFLUX_API_TOKEN is present but empty, stdio treats it as
// missing.
func TestLoadExplicitEmptyAPITokenStdioRejected(t *testing.T) {
	setEnv(t, map[string]string{
		"MINIFLUX_API_URL":   "https://reader.example.com",
		"MINIFLUX_API_TOKEN": "",
	})

	_, err := Load("stdio")
	if err == nil {
		t.Fatal("Load(\"stdio\") = nil error with empty MINIFLUX_API_TOKEN, want ErrMissingAPIToken")
	}
	if !errors.Is(err, ErrMissingAPIToken) {
		t.Errorf("Load(\"stdio\") error = %v, want ErrMissingAPIToken", err)
	}
}

// TestLoadMissingAPITokenHTTPAllowed locks the mode-aware exception: in http
// mode the token is OPTIONAL (auth via inbound X-Auth-Token pass-through /
// runtime fallback, SPEC §3.1), so a missing token must NOT error. The loaded
// config is valid, the token field stays empty, and defaults still apply.
func TestLoadMissingAPITokenHTTPAllowed(t *testing.T) {
	setEnv(t, map[string]string{
		"MINIFLUX_API_URL": "https://reader.example.com",
	})

	cfg, err := Load("http")
	if err != nil {
		t.Fatalf("Load(\"http\") returned error, want nil (token optional): %v", err)
	}
	if cfg.MinifluxAPIToken != "" {
		t.Errorf("MinifluxAPIToken = %q, want empty (optional in http mode)", cfg.MinifluxAPIToken)
	}
	if cfg.MinifluxAPIURL != "https://reader.example.com" {
		t.Errorf("MinifluxAPIURL = %q, want %q", cfg.MinifluxAPIURL, "https://reader.example.com")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want default %q", cfg.ListenAddr, ":8080")
	}
	if cfg.InternalAddr != ":8081" {
		t.Errorf("InternalAddr = %q, want default %q", cfg.InternalAddr, ":8081")
	}
}

// TestLoadExplicitEmptyAPITokenHTTPAllowed is the boundary of the http-mode
// exception: even an explicitly-empty token is allowed in http mode (no error).
func TestLoadExplicitEmptyAPITokenHTTPAllowed(t *testing.T) {
	setEnv(t, map[string]string{
		"MINIFLUX_API_URL":   "https://reader.example.com",
		"MINIFLUX_API_TOKEN": "",
	})

	cfg, err := Load("http")
	if err != nil {
		t.Fatalf("Load(\"http\") returned error with empty token, want nil: %v", err)
	}
	if cfg.MinifluxAPIToken != "" {
		t.Errorf("MinifluxAPIToken = %q, want empty", cfg.MinifluxAPIToken)
	}
}

// TestLoadSentinelErrorsAreDistinct locks that the two sentinel errors are not
// the same value, so callers can distinguish a missing URL from a missing token.
func TestLoadSentinelErrorsAreDistinct(t *testing.T) {
	if ErrMissingAPIURL == ErrMissingAPIToken {
		t.Fatal("ErrMissingAPIURL and ErrMissingAPIToken must be distinct sentinel errors")
	}
}

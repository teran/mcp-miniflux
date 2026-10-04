package domain

import (
	"reflect"
	"testing"
)

// CONTRACT — `domain.Config` (SPEC §6.4, §9 table; S02).
//
// The developer must define the following in package `domain` (file
// domain/config.go). This test file is written FIRST (TDD, red state) and will
// not compile until the struct and its methods exist. The exact contract:
//
//	type Config struct {
//		MinifluxAPIURL   string `envconfig:"MINIFLUX_API_URL"`             // REQUIRED, no default
//		MinifluxAPIToken string `envconfig:"MINIFLUX_API_TOKEN" secret:"true"` // REQUIRED, secret
//		ListenAddr       string `envconfig:"LISTEN_ADDR"`                  // default ":8080"
//		InternalAddr     string `envconfig:"INTERNAL_ADDR"`                // default ":8081"
//		LogLevel         string `envconfig:"LOG_LEVEL"`                    // default "info"
//		LogFormat        string `envconfig:"LOG_FORMAT"`                   // default "text"
//		LogFilename      string `envconfig:"LOG_FILENAME"`                 // no default
//	}
//
//	// SetDefaults fills the documented defaults for fields that are still
//	// empty (so explicitly configured values win). Implemented as a method so a
//	// zero-value Config{} represents the "everything unset" state and applying
//	// SetDefaults yields the documented defaults.
//	func (c *Config) SetDefaults()
//
//	// SecretFields returns the names of all fields annotated `secret:"true"`,
//	// in struct field order. Drives the S02 redaction helper.
//	func (c Config) SecretFields() []string
//
// DESIGN DECISION (documented): `Mode` is intentionally NOT a field on Config.
// Per SPEC §3 (M06) the launch mode `-mode http|stdio` is a command-line flag
// (default `stdio`), not an environment variable. Config is purely env-driven
// via envconfig; keeping Mode off the struct keeps the env-config surface and
// the flag surface disjoint. The `-mode` flag lives in cmd/mcp-miniflux and is
// out of scope for the domain.Config model. Consequently there is no Mode field
// and no mode setter here.

// TestConfigEnvTags locks the envconfig tag contract (§9 table). This is the
// mapping the infrastructure loader relies on; changing a tag name silently
// breaks env resolution.
func TestConfigEnvTags(t *testing.T) {
	cfg := Config{}
	typ := reflect.TypeOf(cfg)

	want := map[string]struct {
		env    string
		secret bool
	}{
		"MinifluxAPIURL":   {env: "MINIFLUX_API_URL", secret: false},
		"MinifluxAPIToken": {env: "MINIFLUX_API_TOKEN", secret: true},
		"ListenAddr":       {env: "LISTEN_ADDR", secret: false},
		"InternalAddr":     {env: "INTERNAL_ADDR", secret: false},
		"LogLevel":         {env: "LOG_LEVEL", secret: false},
		"LogFormat":        {env: "LOG_FORMAT", secret: false},
		"LogFilename":      {env: "LOG_FILENAME", secret: false},
	}

	if got := typ.NumField(); got != len(want) {
		t.Fatalf("Config has %d fields, want %d (only the env-driven fields; Mode must stay off Config)", got, len(want))
	}

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		exp, ok := want[f.Name]
		if !ok {
			t.Errorf("unexpected Config field %q — only %v are allowed", f.Name, keysOf(want))
			continue
		}
		if got := f.Tag.Get("envconfig"); got != exp.env {
			t.Errorf("field %s: envconfig tag = %q, want %q", f.Name, got, exp.env)
		}
		if got := f.Tag.Get("secret"); (got == "true") != exp.secret {
			t.Errorf("field %s: secret tag = %q, want secret=%v", f.Name, got, exp.secret)
		}
	}
}

// TestConfigSecretTagIsPresent locks the S02 annotation that drives redaction.
// If this tag is removed the redaction helper can no longer find the API token
// and a secret would leak into logs/output. Must never regress.
func TestConfigSecretTagIsPresent(t *testing.T) {
	f, ok := reflect.TypeOf(Config{}).FieldByName("MinifluxAPIToken")
	if !ok {
		t.Fatal("Config.MinifluxAPIToken field not found")
	}
	if got := f.Tag.Get("secret"); got != "true" {
		t.Fatalf("Config.MinifluxAPIToken secret tag = %q, want \"true\"", got)
	}
}

// TestSecretFields enumerates the exact secret-annotated field set. The
// redaction helper (S02) uses this; any deviation changes what is redacted.
func TestSecretFields(t *testing.T) {
	got := Config{}.SecretFields()
	want := []string{"MinifluxAPIToken"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SecretFields() = %v, want %v", got, want)
	}
}

// TestSetDefaults verifies a zero-value Config{} resolves to the documented
// defaults from the §9 table. A zero-value Config{} is the "all env unset"
// state; SetDefaults() fills the defaults so the loader always ends up with a
// runnable configuration.
func TestSetDefaults(t *testing.T) {
	c := Config{}
	c.SetDefaults()

	if c.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", c.ListenAddr, ":8080")
	}
	if c.InternalAddr != ":8081" {
		t.Errorf("InternalAddr = %q, want %q", c.InternalAddr, ":8081")
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", c.LogLevel, "info")
	}
	if c.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", c.LogFormat, "text")
	}

	// Fields without defaults must remain empty so the loader can detect
	// "required but missing" (MINIFLUX_API_URL / MINIFLUX_API_TOKEN).
	if c.MinifluxAPIURL != "" {
		t.Errorf("MinifluxAPIURL = %q, want empty (no default)", c.MinifluxAPIURL)
	}
	if c.MinifluxAPIToken != "" {
		t.Errorf("MinifluxAPIToken = %q, want empty (no default)", c.MinifluxAPIToken)
	}
	if c.LogFilename != "" {
		t.Errorf("LogFilename = %q, want empty (no default)", c.LogFilename)
	}
}

// TestSetDefaultsPreservesExplicitValues guards precedence: an already
// populated (env-provided) value must NOT be clobbered by SetDefaults.
func TestSetDefaultsPreservesExplicitValues(t *testing.T) {
	c := Config{
		ListenAddr:   ":9090",
		InternalAddr: ":9091",
		LogLevel:     "debug",
		LogFormat:    "json",
	}
	c.SetDefaults()

	if c.ListenAddr != ":9090" {
		t.Errorf("explicit ListenAddr clobbered by SetDefaults: got %q", c.ListenAddr)
	}
	if c.InternalAddr != ":9091" {
		t.Errorf("explicit InternalAddr clobbered by SetDefaults: got %q", c.InternalAddr)
	}
	if c.LogLevel != "debug" {
		t.Errorf("explicit LogLevel clobbered by SetDefaults: got %q", c.LogLevel)
	}
	if c.LogFormat != "json" {
		t.Errorf("explicit LogFormat clobbered by SetDefaults: got %q", c.LogFormat)
	}
}

// TestSetDefaultsIdempotent: applying defaults twice must be a no-op (stable
// against repeated application in the loader).
func TestSetDefaultsIdempotent(t *testing.T) {
	c := Config{}
	c.SetDefaults()
	first := c
	c.SetDefaults()
	if !reflect.DeepEqual(c, first) {
		t.Fatalf("SetDefaults not idempotent: before=%+v after=%+v", first, c)
	}
}

func keysOf(m map[string]struct {
	env    string
	secret bool
}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

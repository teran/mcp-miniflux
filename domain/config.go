package domain

import "reflect"

// Config is the env-driven configuration model (SPEC §6.4, §9 table; S02).
//
// Config is purely env-driven via envconfig; the launch mode `-mode http|stdio`
// is intentionally NOT a field here (SPEC §3, M06) — it is a command-line flag
// that lives in cmd/mcp-miniflux, keeping the env-config surface and the flag
// surface disjoint.
type Config struct {
	MinifluxAPIURL   string `envconfig:"MINIFLUX_API_URL"`                 // REQUIRED, no default
	MinifluxAPIToken string `envconfig:"MINIFLUX_API_TOKEN" secret:"true"` // REQUIRED, secret
	ListenAddr       string `envconfig:"LISTEN_ADDR"`                      // default ":8080"
	InternalAddr     string `envconfig:"INTERNAL_ADDR"`                    // default ":8081"
	LogLevel         string `envconfig:"LOG_LEVEL"`                        // default "info"
	LogFormat        string `envconfig:"LOG_FORMAT"`                       // default "text"
	LogFilename      string `envconfig:"LOG_FILENAME"`                     // no default
}

// SetDefaults fills the documented defaults for fields that are still empty, so
// explicitly configured (env-provided) values win. It is idempotent: applying
// it repeatedly is a no-op. Fields without a documented default (API URL, API
// token, log filename) are left empty so the loader can detect "required but
// missing".
func (c *Config) SetDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.InternalAddr == "" {
		c.InternalAddr = ":8081"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "text"
	}
}

// SecretFields returns the names of all fields annotated `secret:"true"`, in
// struct field order. It drives the S02 redaction helper.
func (c Config) SecretFields() []string {
	typ := reflect.TypeOf(c)
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Tag.Get("secret") == "true" {
			out = append(out, f.Name)
		}
	}
	return out
}

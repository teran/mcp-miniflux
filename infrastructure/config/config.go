// Package config loads environment-driven configuration (via envconfig) into
// the domain Config model.
package config

import (
	"errors"

	"github.com/kelseyhightower/envconfig"

	"github.com/teran/mcp-miniflux/domain"
)

// Sentinel errors for required-but-missing configuration fields.
var (
	// ErrMissingAPIURL is returned when MINIFLUX_API_URL is absent/empty. It has
	// no default and is REQUIRED in both http and stdio modes (SPEC §6.4, §9).
	ErrMissingAPIURL = errors.New("MINIFLUX_API_URL is required")
	// ErrMissingAPIToken is returned when MINIFLUX_API_TOKEN is absent/empty in
	// stdio mode (secret:true, no default). In http mode the token is optional
	// (auth is provided by the inbound X-Auth-Token pass-through, SPEC §3.1).
	ErrMissingAPIToken = errors.New("MINIFLUX_API_TOKEN is required in stdio mode")
)

// Load reads the environment (via envconfig) into a fresh domain.Config, applies
// cfg.SetDefaults() so explicitly-configured env values win over the documented
// defaults, then runs mode-aware validation.
//
// Validation rules (SPEC §6.4, §9; §3.1):
//   - MINIFLUX_API_URL absent/empty -> ErrMissingAPIURL in BOTH modes.
//   - MINIFLUX_API_TOKEN absent/empty -> ErrMissingAPIToken in stdio mode.
//   - MINIFLUX_API_TOKEN absent/empty in http mode -> NO error (optional).
//
// mode is one of "http"|"stdio". Any unknown mode is treated conservatively like
// stdio (i.e. the token is required), so a typo'd flag cannot silently run with
// an optional token in a mode that expects one. Any envconfig parse error is
// returned as-is.
//
// The resolved token is never logged or printed (L05).
func Load(mode string) (domain.Config, error) {
	var cfg domain.Config
	if err := envconfig.Process("", &cfg); err != nil {
		return domain.Config{}, err
	}

	cfg.SetDefaults()

	if cfg.MinifluxAPIURL == "" {
		return domain.Config{}, ErrMissingAPIURL
	}

	// The token is required only when running in stdio mode (or an unknown
	// mode, which is treated conservatively as stdio). In http mode it is
	// optional: auth comes from the inbound X-Auth-Token pass-through or the
	// runtime fallback (SPEC §3.1).
	if mode != "http" && cfg.MinifluxAPIToken == "" {
		return domain.Config{}, ErrMissingAPIToken
	}

	return cfg, nil
}

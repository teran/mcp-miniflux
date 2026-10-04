// Package config loads environment-driven configuration (via envconfig) into
// the domain Config model.
//
// envconfig is blank-imported here only to pin the exact dependency version in
// go.mod while the package skeleton is empty; real usage lands with the config
// loader in a later phase.
package config

import _ "github.com/kelseyhightower/envconfig"

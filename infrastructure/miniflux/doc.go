// Package miniflux implements the Miniflux HTTP client (resty v3), including
// the X-Auth-Token pass-through header and the secret-redaction helper.
//
// resty is blank-imported here only to pin the exact dependency version in
// go.mod while the package skeleton is empty; real usage lands with the HTTP
// client in a later phase.
package miniflux

import _ "resty.dev/v3"

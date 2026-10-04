// Package logging builds the logrus logger and provides the slog->logrus
// adapter used to wire the go-sdk logger into the server's logging pipeline.
//
// logrus is blank-imported here only to pin the exact dependency version in
// go.mod while the package skeleton is empty; real usage lands with the logger
// builder in a later phase.
package logging

import _ "github.com/sirupsen/logrus"

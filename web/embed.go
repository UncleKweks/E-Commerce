// Package web holds the browser playground: a single HTML page that uses the
// API the way a real frontend would, and shows every request it makes.
//
// The page is compiled into the Go binary with go:embed, so bin/api.exe can
// serve it from any folder without the file being next to it.
package web

import _ "embed" // required for //go:embed

// IndexHTML is the playground page, served at / in debug mode.
//
//go:embed index.html
var IndexHTML []byte

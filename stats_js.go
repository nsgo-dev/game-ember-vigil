//go:build js && wasm

package main

import (
	"net"
	"strings"
	"syscall/js"
)

// statsPage returns the page's host and the referrer it was opened from, and whether the host is a public one: a local or private server reports nothing.
func statsPage() (host, referrer string, ok bool) {
	host = js.Global().Get("location").Get("hostname").String()
	referrer = js.Global().Get("document").Get("referrer").String()
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".local") {
		return host, referrer, false
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return host, referrer, false
	}
	return host, referrer, true
}

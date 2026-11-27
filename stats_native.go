//go:build !(js && wasm)

package main

// statsPage returns the host a native build reports as, the web version's, and no referrer.
func statsPage() (host, referrer string, ok bool) {
	return "nsgo-dev.github.io", "", true
}

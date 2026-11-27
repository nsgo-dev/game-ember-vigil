package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// The game counts its use in Umami through its /api/send, as its web script would: a pageview on launch and on each run's start, an event when a run ends, and a heartbeat each statsBeat while a run is on, for Umami to measure play. It is tagged "app" on the native targets, which report as the Pages host, and "web" in the browser. A native install carries a random ID in storage, so that one install is one visitor. Builds of nsgo dev (statsBuild), test binaries and a local web server (statsPage) report nothing, and the settings turn it off.
const (
	statsURL     = "https://umami.oneo.me/api/send"
	statsWebsite = "16e3f11f-ef74-4ef5-880a-44ecd19ba589"
	statsBeat    = time.Minute
	installKey   = "installID"
)

// statsPayload is an event of Umami's /api/send: a pageview when Name is empty.
type statsPayload struct {
	Website  string `json:"website"`
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Language string `json:"language,omitempty"`
	Screen   string `json:"screen,omitempty"`
	Referrer string `json:"referrer,omitempty"`
	Tag      string `json:"tag"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
}

// usageStats makes the game's events on the graph's goroutine and sends them from another, oldest first; one that fails or overflows the queue is dropped. A nil usageStats reports nothing.
type usageStats struct {
	cx                      *core.Scope
	on                      func() bool
	host, referrer, tag, id string
	queue                   chan statsPayload
	url, title              string
}

// newStats starts reporting while on says the settings allow it, or returns nil where this build or page reports nothing.
func newStats(cx *core.Scope, on func() bool) *usageStats {
	if !statsBuild || testing.Testing() {
		return nil
	}
	host, referrer, ok := statsPage()
	if !ok {
		return nil
	}
	sys := cx.Env().OS()
	if cx.Env().Simulated() {
		sys = core.OSWeb
	}
	s := &usageStats{cx: cx, on: on, host: host, referrer: referrer, tag: "web", queue: make(chan statsPayload, 32)}
	if sys != core.OSWeb {
		s.tag, s.id = "app", installID(cx.Storage())
	}
	go sendStats(s.queue, statsAgent(sys))
	return s
}

// page counts a pageview of url, which later events and heartbeats belong to.
func (s *usageStats) page(url, title string) {
	if s == nil {
		return
	}
	s.url, s.title = url, title
	s.send("")
}

// event counts the named event on the current page.
func (s *usageStats) event(name string) {
	if s != nil {
		s.send(name)
	}
}

// beat sends a heartbeat each statsBeat while playing says a run is on.
func (s *usageStats) beat(playing func() bool) {
	if s == nil {
		return
	}
	var tick func()
	tick = func() {
		if playing() {
			s.send("heartbeat")
		}
		s.cx.AfterFunc(statsBeat, tick)
	}
	s.cx.AfterFunc(statsBeat, tick)
}

func (s *usageStats) send(name string) {
	if !s.on() {
		return
	}
	env := s.cx.Env()
	w := env.Window()
	p := statsPayload{Website: statsWebsite, Hostname: s.host, URL: s.url, Title: s.title,
		Language: env.Locale(), Screen: fmt.Sprintf("%dx%d", int(w.Width), int(w.Height)),
		Referrer: s.referrer, Tag: s.tag, ID: s.id, Name: name}
	// The visit came from the referrer once, with its first page.
	s.referrer = ""
	select {
	case s.queue <- p:
	default:
	}
}

// installID returns the install's random ID, made on first use.
func installID(store core.Storage) string {
	id, ok := store.GetItem(installKey)
	if !ok || id == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		id = hex.EncodeToString(b)
		_ = store.SetItem(installKey, id)
	}
	return id
}

// sendStats posts the queue's events to Umami until it closes.
func sendStats(queue <-chan statsPayload, agent string) {
	client := &http.Client{Timeout: 10 * time.Second}
	for p := range queue {
		body, _ := json.Marshal(map[string]any{"type": "event", "payload": p})
		req, err := http.NewRequest(http.MethodPost, statsURL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if agent != "" {
			req.Header.Set("User-Agent", agent)
		}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
}

// statsAgent is the User-Agent a native build sends, as a browser on the platform would say it, for Umami to tell the system and not take it for a bot as it does Go's own. The browser sends its own.
func statsAgent(os core.OS) string {
	platforms := map[core.OS]string{
		core.OSMacOS:   "Macintosh; Intel Mac OS X 10_15_7",
		core.OSWindows: "Windows NT 10.0; Win64; x64",
		core.OSLinux:   "X11; Linux x86_64",
		core.OSIOS:     "iPhone; CPU iPhone OS 18_0 like Mac OS X",
		core.OSAndroid: "Linux; Android 15",
	}
	p, ok := platforms[os]
	if !ok {
		return ""
	}
	return "Mozilla/5.0 (" + p + ") EmberVigil/1.0"
}

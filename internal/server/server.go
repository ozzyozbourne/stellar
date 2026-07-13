// Package server hosts the editor SPA and a Datastar SSE live-refresh stream
// (stellar.md §7). On every config change it regenerates stellar.css, content-
// hashes it, and patches the consuming page's <link id="stellar-css"> plus the
// editor's size meter. Writes (/config) return 204; all reads arrive over the
// long-lived /live-refresh stream (CQRS, the Tao of Datastar).
package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
	"github.com/starfederation/stellar/internal/generate"
	"github.com/starfederation/stellar/internal/minify"
)

//go:embed assets
var assets embed.FS

// keepHashes is how many old stylesheet versions stay servable so pages that
// haven't yet processed a live-refresh patch never fetch a mismatched body.
const keepHashes = 8

// build is one generated stylesheet: content, hash, and size-meter stats.
type build struct {
	css     string
	hash    string
	tokens  int
	bytes   int
	gzipped int
}

// Server holds in-memory config + generated CSS and a set of SSE subscribers.
type Server struct {
	mu       sync.RWMutex
	cfg      config.Config
	cfgPath  string
	demoDir  string
	cur      build
	history  map[string]string // hash → css for recent builds
	order    []string          // history eviction order
	subs     map[chan struct{}]struct{}
	saveTick *time.Timer
}

// New builds a server from an initial config. demoDir, when it exists on
// disk, is mounted at /demo/ as the consumer smoke-test page.
func New(cfg config.Config, cfgPath string) *Server {
	_ = cfg.ResolveExtraction()
	s := &Server{
		cfg: cfg, cfgPath: cfgPath, demoDir: "demo",
		history: map[string]string{},
		subs:    map[chan struct{}]struct{}{},
	}
	s.regenerate()
	return s
}

var tokenRe = regexp.MustCompile(`--[A-Za-z0-9-]+\s*:`)

// regenerate rebuilds CSS, hash, and meter stats from the current config.
func (s *Server) regenerate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	css := generate.Generate(s.cfg)
	if s.cfg.Output.Mode == "compact" {
		if min, err := minify.CSS(css); err == nil {
			css = min
		}
	}
	sum := sha256.Sum256([]byte(css))
	b := build{
		css:     css,
		hash:    fmt.Sprintf("%x", sum[:6]),
		tokens:  len(tokenRe.FindAllString(css, -1)),
		bytes:   len(css),
		gzipped: gzippedSize(css),
	}
	if _, ok := s.history[b.hash]; !ok {
		s.history[b.hash] = b.css
		s.order = append(s.order, b.hash)
		for len(s.order) > keepHashes {
			delete(s.history, s.order[0])
			s.order = s.order[1:]
		}
	}
	s.cur = b
}

func gzippedSize(css string) int {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(css))
	_ = zw.Close()
	return buf.Len()
}

// Handler wires all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/assets/css/stellar/", s.handleCSS)
	mux.HandleFunc("/live-refresh", s.handleLiveRefresh)
	mux.HandleFunc("/config", s.handleConfig)
	mux.HandleFunc("/config.json", s.handleConfigJSON)
	mux.HandleFunc("/reset", s.handleReset)
	mux.HandleFunc("/extract", s.handleExtract)
	mux.HandleFunc("/demo/", s.handleDemo)
	// Static editor assets (datastar.js etc.).
	sub, _ := fs.Sub(assets, "assets")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	return mux
}

// Listen serves on addr.
func (s *Server) Listen(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) snapshot() (config.Config, build) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg, s.cur
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	cfg, cur := s.snapshot()
	page, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// First paint already has the stylesheet, the config signals, and the meter.
	html := injectHash(string(page), cur.hash)
	html = injectSignals(html, cfg)
	html = injectMeter(html, cur)
	_, _ = w.Write([]byte(html))
}

// handleCSS serves generated stylesheets by content hash (immutable), the
// `current` alias (uncacheable, used by the Export link), and redirects
// unknown hashes to the current build.
func (s *Server) handleCSS(w http.ResponseWriter, r *http.Request) {
	_, cur := s.snapshot()
	name := path.Base(r.URL.Path)
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	switch {
	case name == "current":
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(cur.css))
	case name == cur.hash:
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		_, _ = w.Write([]byte(cur.css))
	default:
		s.mu.RLock()
		css, ok := s.history[name]
		s.mu.RUnlock()
		if ok {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			_, _ = w.Write([]byte(css))
			return
		}
		http.Redirect(w, r, "/assets/css/stellar/"+cur.hash, http.StatusFound)
	}
}

// handleDemo serves the consumer smoke-test page from ./demo. A missing
// demo/stellar.css falls back to the live generated CSS so the page renders
// before any `stellar build` has run.
func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/demo/")
	if rel == "" {
		rel = "index.html"
	}
	fp := path.Join(s.demoDir, path.Clean("/"+rel))
	if _, err := os.Stat(fp); err != nil {
		if rel == "stellar.css" {
			_, cur := s.snapshot()
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(cur.css))
			return
		}
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fp)
}

// clone deep-copies a config through JSON so a failed decode can never leave
// the live config half-mutated (maps/slices are not shared).
func clone(c config.Config) (config.Config, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return config.Config{}, err
	}
	var out config.Config
	if err := json.Unmarshal(b, &out); err != nil {
		return config.Config{}, err
	}
	return out, nil
}

// handleConfig accepts the editor's signals (a full or partial JSON config
// patch), regenerates, and returns 204: it is a command — every read arrives
// over /live-refresh (CQRS).
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	next, err := clone(s.cfg)
	prevHash := s.cur.hash
	s.mu.RUnlock()
	if err == nil {
		err = json.NewDecoder(r.Body).Decode(&next)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = next.ResolveExtraction()
	s.mu.Lock()
	s.cfg = next
	s.mu.Unlock()

	s.regenerate()
	s.scheduleSave()
	if _, cur := s.snapshot(); cur.hash != prevHash {
		s.broadcast()
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleReset restores the default config ("Reset defaults" in the editor).
// Unlike /config it answers with an SSE body: a signals patch pushes the
// default values back into every bound input, plus the fresh link + meter so
// the resetting editor restyles without waiting on its /live-refresh stream.
// Other subscribers converge through the normal broadcast.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	s.cfg = config.Default()
	s.mu.Unlock()
	s.regenerate()
	s.scheduleSave()
	s.broadcast()

	cfg, cur := s.snapshot()
	w.Header().Set("Content-Type", "text/event-stream")
	writeSignalsPatch(w, cfg)
	writeElementPatch(w, linkElement(cur.hash))
	writeElementPatch(w, meterElement(cur))
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
}

func (s *Server) handleConfigJSON(w http.ResponseWriter, r *http.Request) {
	cfg, _ := s.snapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// handleExtract accepts a multipart image upload (Datastar contentType:'form')
// and answers with an SSE patch that renders the candidate swatches. Each
// swatch button writes the harmony-derived role seeds into the config signals
// and re-posts /config — the backend stays the only source of truth (§5.4).
func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	f, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image field: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()
	img, err := color.DecodeImage(f)
	if err != nil {
		http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
		return
	}
	cands := color.Extract(img, 8)
	mode := r.FormValue("harmony")
	if mode == "" {
		mode = "complement"
	}
	w.Header().Set("Content-Type", "text/event-stream")
	writeElementPatch(w, candidatesElement(cands, mode))
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
}

// handleLiveRefresh is a long-lived Datastar SSE stream. It immediately sends
// the current stylesheet link + meter, then re-sends on every change.
func (s *Server) handleLiveRefresh(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// A 1-buffered signal channel coalesces bursts: the handler always
	// snapshots the latest build, so a pending signal is never stale.
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()

	// Only the editor asks for size-meter patches (?meter=1); consumer pages
	// subscribe plainly and receive just the stylesheet link.
	withMeter := r.URL.Query().Get("meter") != ""
	send := func() {
		_, cur := s.snapshot()
		writePatches(w, flusher, cur, withMeter)
	}
	send()

	ctx := r.Context()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			send()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) broadcast() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // a change signal is already pending
		}
	}
}

// scheduleSave debounces persisting the config to disk (§7 step 3).
func (s *Server) scheduleSave() {
	if s.cfgPath == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveTick != nil {
		s.saveTick.Stop()
	}
	cfg := s.cfg
	path := s.cfgPath
	s.saveTick = time.AfterFunc(500*time.Millisecond, func() {
		_ = config.Save(path, cfg)
	})
}

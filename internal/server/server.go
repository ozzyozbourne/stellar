// Package server hosts the editor SPA and a Datastar SSE live-refresh stream
// (stellar.md §7). On every config change it regenerates stellar.css, content-
// hashes it, and patches the consuming page's <link id="stellar-css">.
package server

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
	"github.com/starfederation/stellar/internal/generate"
	"github.com/starfederation/stellar/internal/minify"
)

//go:embed assets
var assets embed.FS

// Server holds in-memory config + generated CSS and a set of SSE subscribers.
type Server struct {
	mu       sync.RWMutex
	cfg      config.Config
	cfgPath  string
	css      string
	hash     string
	subs     map[chan string]struct{}
	saveTick *time.Timer
}

// New builds a server from an initial config.
func New(cfg config.Config, cfgPath string) *Server {
	_ = cfg.ResolveExtraction()
	s := &Server{cfg: cfg, cfgPath: cfgPath, subs: map[chan string]struct{}{}}
	s.regenerate()
	return s
}

// regenerate rebuilds CSS + hash from the current config (caller holds no lock
// for read; takes the write lock internally).
func (s *Server) regenerate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	css := generate.Generate(s.cfg)
	if s.cfg.Output.Mode == "compact" {
		if min, err := minify.CSS(css); err == nil {
			css = min
		}
	}
	s.css = css
	sum := sha256.Sum256([]byte(s.css))
	s.hash = fmt.Sprintf("%x", sum[:6])
}

// Listen wires routes and serves.
func (s *Server) Listen(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/assets/css/stellar/", s.handleCSS)
	mux.HandleFunc("/live-refresh", s.handleLiveRefresh)
	mux.HandleFunc("/config", s.handleConfig)
	mux.HandleFunc("/config.json", s.handleConfigJSON)
	mux.HandleFunc("/extract", s.handleExtract)
	// Static editor assets (datastar.js etc.).
	sub, _ := fs.Sub(assets, "assets")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	return http.ListenAndServe(addr, mux)
}

func (s *Server) snapshot() (cfg config.Config, css, hash string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg, s.css, s.hash
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _, hash := s.snapshot()
	page, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Inject the current hash so first paint already has the stylesheet.
	html := injectHash(string(page), hash)
	_, _ = w.Write([]byte(html))
}

func (s *Server) handleCSS(w http.ResponseWriter, r *http.Request) {
	_, css, _ := s.snapshot()
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	// Hashed URL → immutable, long cache (§7).
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write([]byte(css))
}

// handleConfig accepts a full or partial JSON config patch, regenerates, and
// notifies subscribers.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&s.cfg); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = s.cfg.ResolveExtraction()
	s.mu.Unlock()

	s.regenerate()
	s.scheduleSave()
	_, _, hash := s.snapshot()
	s.broadcast(hash)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"hash": hash})
}

func (s *Server) handleConfigJSON(w http.ResponseWriter, r *http.Request) {
	cfg, _, _ := s.snapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

// handleExtract accepts a multipart image upload and returns palette
// candidates plus harmony-derived seeds (§5.4). Query param ?harmony=… sets
// the harmony mode for the returned seeds (default complement).
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
	mode := r.URL.Query().Get("harmony")
	if mode == "" {
		mode = "complement"
	}
	var seeds []string
	if len(cands) > 0 {
		for _, sd := range color.Harmony(cands[0].Color, mode) {
			seeds = append(seeds, sd.CSS())
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"candidates": cands,
		"harmony":    seeds,
	})
}

// handleLiveRefresh is a long-lived Datastar SSE stream. It immediately sends
// the current stylesheet link, then pushes a new one on every change.
func (s *Server) handleLiveRefresh(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 4)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
		close(ch)
	}()

	_, _, hash := s.snapshot()
	writeLinkPatch(w, flusher, hash)

	ctx := r.Context()
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case h := <-ch:
			writeLinkPatch(w, flusher, h)
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) broadcast(hash string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- hash:
		default:
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

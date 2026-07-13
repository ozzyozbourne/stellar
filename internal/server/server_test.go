package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/starfederation/stellar/internal/config"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := New(config.Default(), "") // no cfgPath → no disk writes
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func get(t *testing.T, c *http.Client, url string) *http.Response {
	t.Helper()
	res, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()
	var b strings.Builder
	if _, err := readAll(&b, res); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func readAll(b *strings.Builder, res *http.Response) (int64, error) {
	buf := make([]byte, 32*1024)
	var n int64
	for {
		m, err := res.Body.Read(buf)
		b.Write(buf[:m])
		n += int64(m)
		if err != nil {
			if err.Error() == "EOF" {
				return n, nil
			}
			return n, err
		}
	}
}

// §7: hashed URLs are immutable; `current` is uncacheable; old hashes stay
// servable after a config change; unknown hashes redirect to the current one.
func TestHashedCSSServing(t *testing.T) {
	s, ts := newTestServer(t)
	_, first := s.snapshot()

	res := get(t, ts.Client(), ts.URL+"/assets/css/stellar/"+first.hash)
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("hashed URL not immutable: %q", cc)
	}
	firstCSS := body(t, res)

	res = get(t, ts.Client(), ts.URL+"/assets/css/stellar/current")
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("current alias should be no-store, got %q", cc)
	}

	// Change the config → new hash.
	post, err := ts.Client().Post(ts.URL+"/config", "application/json",
		strings.NewReader(`{"size":{"minRatio":1.2}}`))
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /config = %d, want 204", post.StatusCode)
	}
	_, second := s.snapshot()
	if second.hash == first.hash {
		t.Fatal("config change did not change the hash")
	}

	// The old hash must still serve the old bytes.
	res = get(t, ts.Client(), ts.URL+"/assets/css/stellar/"+first.hash)
	if got := body(t, res); got != firstCSS {
		t.Error("old hashed URL no longer serves its original content")
	}

	// Unknown hash → redirect to the current build.
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res = get(t, noRedirect, ts.URL+"/assets/css/stellar/deadbeef0000")
	if res.StatusCode != http.StatusFound {
		t.Errorf("unknown hash = %d, want 302", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); !strings.HasSuffix(loc, second.hash) {
		t.Errorf("redirect location %q does not point at current hash %s", loc, second.hash)
	}
}

// A bad patch must not leave the live config half-mutated.
func TestConfigDecodeAtomic(t *testing.T) {
	s, ts := newTestServer(t)
	before, _ := s.snapshot()

	res, err := ts.Client().Post(ts.URL+"/config", "application/json",
		strings.NewReader(`{"size":{"minRatio":2},"viewport":{"min":"oops"}}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad patch = %d, want 400", res.StatusCode)
	}
	after, _ := s.snapshot()
	if after.Size.MinRatio != before.Size.MinRatio {
		t.Errorf("failed decode mutated config: minRatio %v → %v",
			before.Size.MinRatio, after.Size.MinRatio)
	}
}

// POST /reset restores config.Default() and answers with a signals patch so
// every bound editor input updates.
func TestReset(t *testing.T) {
	s, ts := newTestServer(t)

	res, err := ts.Client().Post(ts.URL+"/config", "application/json",
		strings.NewReader(`{"size":{"minRatio":1.3}}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if cfg, _ := s.snapshot(); cfg.Size.MinRatio != 1.3 {
		t.Fatal("config change did not apply")
	}

	res, err = ts.Client().Post(ts.URL+"/reset", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := body(t, res)
	if !strings.Contains(stream, "datastar-patch-signals") {
		t.Error("reset response missing signals patch")
	}
	if !strings.Contains(stream, "datastar-patch-elements") {
		t.Error("reset response missing element patches")
	}

	got, _ := s.snapshot()
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(config.Default())
	if string(gotJSON) != string(wantJSON) {
		t.Error("config after reset is not Default()")
	}
}

// The editor page is served with signals, stylesheet hash, and meter injected.
func TestIndexInjection(t *testing.T) {
	s, ts := newTestServer(t)
	_, cur := s.snapshot()
	res := get(t, ts.Client(), ts.URL+"/")
	page := body(t, res)
	if strings.Contains(page, "__STELLAR_SIGNALS__") {
		t.Error("signals placeholder not injected")
	}
	if !strings.Contains(page, cur.hash) {
		t.Error("stylesheet hash not injected")
	}
	if !strings.Contains(page, "tokens · ") {
		t.Error("meter not injected")
	}
	if !strings.Contains(page, "&#34;viewport&#34;") {
		t.Error("config signals not present (html-escaped JSON expected)")
	}
}

// /live-refresh immediately pushes the current link + meter patches.
func TestLiveRefreshInitialPatch(t *testing.T) {
	s, ts := newTestServer(t)
	_, cur := s.snapshot()

	req, _ := http.NewRequest("GET", ts.URL+"/live-refresh?meter=1", nil)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := make([]byte, 4096)
	n, _ := res.Body.Read(buf)
	head := string(buf[:n])
	if !strings.Contains(head, "datastar-patch-elements") {
		t.Error("no patch-elements event in initial stream")
	}
	if !strings.Contains(head, cur.hash) {
		t.Error("initial patch does not carry the current hash")
	}
	if !strings.Contains(head, `id="meter"`) {
		t.Error("initial stream missing meter patch")
	}

	// A plain consumer stream gets only the link — no meter patch it has no
	// target for.
	req, _ = http.NewRequest("GET", ts.URL+"/live-refresh", nil)
	res2, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	n, _ = res2.Body.Read(buf)
	if head := string(buf[:n]); strings.Contains(head, `id="meter"`) {
		t.Error("consumer stream should not receive meter patches")
	}
}

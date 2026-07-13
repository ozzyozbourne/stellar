// Datastar SSE payloads + HTML injection helpers. The backend drives the
// frontend by patching elements (the Tao of Datastar): the stylesheet link,
// the size meter, and the extraction candidates are all server-rendered.
package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
)

// linkElement renders the stylesheet <link> for a given content hash.
func linkElement(hash string) string {
	return fmt.Sprintf(`<link id="stellar-css" rel="stylesheet" href="/assets/css/stellar/%s">`, hash)
}

// meterElement renders the editor's size meter (§8: custom-property count +
// bytes + gzipped bytes). Pages without #meter (e.g. the demo) ignore it.
func meterElement(b build) string {
	return fmt.Sprintf(`<p class="meter" id="meter">%d tokens · %d bytes · %d gzipped</p>`,
		b.tokens, b.bytes, b.gzipped)
}

// writeElementPatch emits one `datastar-patch-elements` SSE event. Datastar
// matches incoming elements to the live DOM by id and morphs in place.
func writeElementPatch(w http.ResponseWriter, elements string) {
	fmt.Fprint(w, "event: datastar-patch-elements\n")
	fmt.Fprint(w, "data: mode outer\n")
	for line := range strings.SplitSeq(elements, "\n") {
		fmt.Fprintf(w, "data: elements %s\n", line)
	}
	fmt.Fprint(w, "\n")
}

// writeSignalsPatch emits one `datastar-patch-signals` SSE event carrying the
// whole config, updating every data-bind'ed editor input — the Tao's "patch
// signals when appropriate" case (used after a reset, where the backend must
// push new values into the form).
func writeSignalsPatch(w http.ResponseWriter, cfg config.Config) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	fmt.Fprint(w, "event: datastar-patch-signals\n")
	fmt.Fprintf(w, "data: signals %s\n", b)
	fmt.Fprint(w, "\n")
}

// writePatches pushes the current stylesheet link (and, for the editor, the
// meter) over an open SSE stream (§7 step 2). The browser swaps the sheet
// with no navigation.
func writePatches(w http.ResponseWriter, f http.Flusher, b build, withMeter bool) {
	writeElementPatch(w, linkElement(b.hash))
	if withMeter {
		writeElementPatch(w, meterElement(b))
	}
	f.Flush()
}

// candidatesElement renders the extraction palette as swatch buttons. Each
// button writes the harmony-derived seeds into the config signals and
// re-posts /config, so picking a candidate is a normal backend write (§5.4).
func candidatesElement(cands []color.Candidate, mode string) string {
	var b strings.Builder
	b.WriteString(`<ul class="swatches" id="candidates">`)
	for _, c := range cands {
		seeds := color.Harmony(c.Color, mode)
		set := []string{fmt.Sprintf("$colors.roles.primary.seed = '%s'", seeds[0].CSS())}
		if len(seeds) > 1 {
			set = append(set, fmt.Sprintf("$colors.roles.secondary.seed = '%s'", seeds[1].CSS()))
		}
		neutral := color.OKLCH{L: seeds[0].L, C: 0.01, H: seeds[0].H}
		set = append(set,
			fmt.Sprintf("$colors.roles.neutral.seed = '%s'", neutral.CSS()),
			"@post('/config')")
		fmt.Fprintf(&b,
			`<li><button type="button" class="swatch" style="background:%s" title="%s" data-on:click="%s">%s</button></li>`,
			html.EscapeString(c.CSS),
			html.EscapeString(fmt.Sprintf("%s · %.0f%%", c.Name, c.Population*100)),
			html.EscapeString(strings.Join(set, "; ")),
			strings.Repeat("★", c.Stars))
	}
	b.WriteString(`</ul>`)
	return b.String()
}

// injectHash replaces the placeholder stylesheet link in the served HTML with
// one carrying the current hash, so the first paint is already styled.
func injectHash(page, hash string) string {
	return strings.Replace(page,
		`<link id="stellar-css" rel="stylesheet">`,
		linkElement(hash), 1)
}

// injectSignals seeds the editor's data-signals with the current config, so
// every input is bound before the first interaction.
func injectSignals(page string, cfg config.Config) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return page
	}
	return strings.Replace(page, "__STELLAR_SIGNALS__", html.EscapeString(string(b)), 1)
}

// injectMeter fills the size meter on first paint.
func injectMeter(page string, b build) string {
	return strings.Replace(page, `<p class="meter" id="meter"></p>`, meterElement(b), 1)
}

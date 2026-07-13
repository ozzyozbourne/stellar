package generate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/starfederation/stellar/internal/config"
)

func TestTokenNames(t *testing.T) {
	css := `:root { --primary-1: red; --primary-1-on: blue; --font-size--2: 1rem; }
[data-theme="dark"] { --primary-1: black; }`
	got := TokenNames(css)
	want := []string{"--primary-1", "--primary-1-on", "--font-size--2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TokenNames = %v, want %v", got, want)
	}
}

func TestFamilyRollup(t *testing.T) {
	cases := map[string]string{
		"--primary-3":       "primary",
		"--primary-3-on":    "primary",
		"--primary-12-dim":  "primary",
		"--font-size--2":    "font-size",
		"--code-bg":         "code-bg",
		"--anim-duration-1": "anim-duration",
		"--border-size-2":   "border-size",
	}
	for tok, want := range cases {
		if got := family(tok); got != want {
			t.Errorf("family(%s) = %q, want %q", tok, got, want)
		}
	}
}

func TestUsage(t *testing.T) {
	defined := []string{"--primary-1", "--primary-2", "--primary-10", "--code-bg"}
	sources := map[string]string{
		"styles.css": `h1 { color: var(--primary-1); background: var(--primary-10); }`,
	}
	rep := Usage(defined, sources)
	if rep.Defined != 4 {
		t.Errorf("Defined = %d, want 4", rep.Defined)
	}
	if !reflect.DeepEqual(rep.Used, []string{"--primary-1", "--primary-10"}) {
		t.Errorf("Used = %v", rep.Used)
	}
	// --primary-1 must not be marked used by the --primary-10 reference, and
	// --primary-2 / --code-bg are unused.
	if !reflect.DeepEqual(rep.Unused, []string{"--primary-2", "--code-bg"}) {
		t.Errorf("Unused = %v", rep.Unused)
	}
	var prim FamilyUsage
	for _, f := range rep.Families {
		if f.Family == "primary" {
			prim = f
		}
	}
	if prim.Total != 3 || prim.Used != 2 {
		t.Errorf("primary family = %+v, want Total 3 Used 2", prim)
	}
}

// End-to-end sanity: the demo stylesheet-style source uses ramp tokens but no
// chart tokens.
func TestUsageAgainstGenerated(t *testing.T) {
	css := Generate(config.Default())
	defined := TokenNames(css)
	if len(defined) < 100 {
		t.Fatalf("expected many tokens, got %d", len(defined))
	}
	rep := Usage(defined, map[string]string{
		"site.css": `body { color: var(--neutral-12); background: var(--neutral-1); }`,
	})
	if len(rep.Used) != 2 {
		t.Errorf("Used = %v, want exactly the two neutral tokens", rep.Used)
	}
	if strings.Join(rep.Used, " ") != "--neutral-1 --neutral-12" {
		t.Errorf("Used = %v", rep.Used)
	}
}

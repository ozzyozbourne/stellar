package generate

import (
	"regexp"
	"strings"
	"testing"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
)

// §11 scale parity: the verified font-size-1 clamp appears verbatim.
func TestScaleParity(t *testing.T) {
	css := Generate(config.Default())
	want := "--font-size-1: clamp(1.125rem, calc(1.103571rem + 0.107143vw), 1.2rem);"
	if !strings.Contains(css, want) {
		t.Errorf("generated CSS missing verified token:\n%s", want)
	}
}

var oklchRe = regexp.MustCompile(`oklch\(([0-9.]+)% ([0-9.]+) ([0-9.]+)\)`)

// §11 gamut: every emitted oklch() maps back into sRGB [0,1].
func TestGamutAllInRange(t *testing.T) {
	css := Generate(config.Default())
	ms := oklchRe.FindAllStringSubmatch(css, -1)
	if len(ms) < 100 {
		t.Fatalf("expected many oklch tokens, got %d", len(ms))
	}
	for _, m := range ms {
		c, err := color.ParseOKLCH(m[0])
		if err != nil {
			t.Fatalf("parse %q: %v", m[0], err)
		}
		rgb := c.ToRGB()
		for _, v := range []float64{rgb.R, rgb.G, rgb.B} {
			if v < -2e-3 || v > 1+2e-3 {
				t.Errorf("%s out of sRGB gamut: %v", m[0], rgb)
				break
			}
		}
	}
}

// maxAchievableLc brute-forces the best |Lc| any greyscale foreground can
// reach against a background — the ceiling FindOnColor is held to.
func maxAchievableLc(bg color.OKLCH) float64 {
	bgY := bg.Clamped().LumaY()
	best := 0.0
	for i := 0; i <= 1000; i++ {
		L := float64(i) / 1000
		fgY := color.GamutMap(color.OKLCH{L: L, C: 0, H: bg.H}).Clamped().LumaY()
		if lc := absf(color.APCAContrast(fgY, bgY)); lc > best {
			best = lc
		}
	}
	return best
}

func absf(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// §11 contrast: for each theme step in both light and dark ramps, the solved
// -on / -dim foregrounds meet the APCA target when it is physically reachable,
// and otherwise hit the maximum achievable contrast for that surface (§12:
// Lc 90 is unreachable against mid-tone greys).
func TestContrastGates(t *testing.T) {
	c := config.Default()
	check := func(role string, step int, invert bool, fg, bg color.OKLCH, label string, tgt float64) {
		got := color.APCABetween(fg, bg)
		max := maxAchievableLc(bg)
		switch {
		case max >= tgt: // target reachable → must be met
			if got < tgt-0.5 {
				t.Errorf("%s-%d (invert=%v) %s Lc %.1f < target %g (reachable, max %.1f)",
					role, step, invert, label, got, tgt, max)
			}
		default: // unreachable → must be within 1 Lc of the ceiling
			if got < max-1.0 {
				t.Errorf("%s-%d (invert=%v) %s Lc %.1f not maximal (ceiling %.1f)",
					role, step, invert, label, got, max)
			}
		}
	}
	for _, role := range []string{"primary", "secondary", "neutral"} {
		for _, invert := range []bool{false, true} {
			for _, tone := range ramp(c.Colors.Roles[role], c.Colors, invert) {
				check(role, tone.Step, invert, tone.On, tone.Bg, "on", c.Colors.APCA.OnLc)
				check(role, tone.Step, invert, tone.Dim, tone.Bg, "dim", c.Colors.APCA.DimLc)
			}
		}
	}
}

// §11 size meter: disabling a section reduces token count + bytes.
func TestSectionToggleReducesOutput(t *testing.T) {
	full := Generate(config.Default())
	c := config.Default()
	c.Sections.Charts = false
	less := Generate(c)
	if !(len(less) < len(full)) {
		t.Errorf("disabling charts did not shrink output: %d !< %d", len(less), len(full))
	}
	count := func(s string) int { return strings.Count(s, "--chart-") }
	if count(less) != 0 || count(full) == 0 {
		t.Errorf("chart token counts wrong: full=%d less=%d", count(full), count(less))
	}
}

// Balanced braces — a cheap "file parses" check (§10 gate B).
func TestBracesBalanced(t *testing.T) {
	css := Generate(config.Default())
	if o, c := strings.Count(css, "{"), strings.Count(css, "}"); o != c {
		t.Errorf("unbalanced braces: %d open, %d close", o, c)
	}
}

// §5.3 + §7 contract: every color partial exposes light in `:root,
// [data-theme="light"]`, dark under the media query gated on
// `:root:not([data-theme="light"])`, and dark again under `[data-theme="dark"]`
// so the data-attr:data-theme toggle actually restyles.
func TestThemeAttrBlocks(t *testing.T) {
	css := Generate(config.Default())
	for _, want := range []string{
		`:root, [data-theme="light"] {`,
		`:root:not([data-theme="light"]) {`,
		`[data-theme="dark"] {`,
		"color-scheme: light;",
		"color-scheme: dark;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("generated CSS missing theme block %q", want)
		}
	}
	// The explicit-dark block must carry the same first token as the media
	// dark block (same builder, both polarities emitted).
	darkAttr := css[strings.Index(css, `[data-theme="dark"]`):]
	if !strings.Contains(darkAttr, "--primary-1:") {
		t.Error(`[data-theme="dark"] block missing role tokens`)
	}
}

// §6 radius / border / animation families.
func TestSimpleFamilies(t *testing.T) {
	css := Generate(config.Default())
	for _, want := range []string{
		"--border-radius-1: 0.25rem;",
		"--border-radius-5: 1.265625rem;",
		"--border-radius-round: 9999px;",
		"--border-size-1: 1px;",
		"--border-size-3: 4px;",
		"--anim-duration-base: 0.3s;",
		"--anim-duration--1: 0.15s;",
		"--anim-duration-2: 1.2s;",
		"--anim-ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("generated CSS missing %q", want)
		}
	}
	c := config.Default()
	c.Sections.Radius, c.Sections.Border, c.Sections.Animation = false, false, false
	less := Generate(c)
	for _, gone := range []string{"--border-radius-", "--border-size-", "--anim-"} {
		if strings.Contains(less, gone) {
			t.Errorf("disabled section still emits %q tokens", gone)
		}
	}
}

// §6 gradients: var() references into the theme ramps, so gradients restyle
// with the ramps; section toggle removes them.
func TestGradients(t *testing.T) {
	css := Generate(config.Default())
	for _, want := range []string{
		"--gradient-primary: linear-gradient(135deg in oklch, var(--primary-4), var(--primary-9));",
		"--gradient-secondary: linear-gradient(135deg in oklch, var(--secondary-4), var(--secondary-9));",
		"--gradient-neutral: linear-gradient(135deg in oklch, var(--neutral-4), var(--neutral-9));",
		"--gradient-brand: linear-gradient(135deg in oklch, var(--primary-6), var(--secondary-6));",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("generated CSS missing %q", want)
		}
	}
	c := config.Default()
	c.Sections.Gradients = false
	if strings.Contains(Generate(c), "--gradient-") {
		t.Error("disabled gradients section still emits tokens")
	}
}

// Flat-rem when min==max (font step 0 pinned to 1rem).
func TestFontStepZeroFlat(t *testing.T) {
	css := Generate(config.Default())
	if !strings.Contains(css, "--font-size-0: 1rem;") {
		t.Error("font-size-0 should be flat 1rem")
	}
}

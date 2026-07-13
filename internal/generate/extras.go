package generate

import (
	"fmt"
	"math"
	"strings"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
)

// neutralSeed returns the configured neutral seed (fallback to a grey).
func neutralSeed(c config.Config) color.OKLCH {
	if r, ok := c.Colors.Roles["neutral"]; ok {
		if oc, err := color.ParseOKLCH(r.Seed); err == nil {
			return oc
		}
	}
	return color.OKLCH{L: 0.63, C: 0.01, H: 230}
}

func primarySeed(c config.Config) color.OKLCH {
	if r, ok := c.Colors.Roles["primary"]; ok {
		if oc, err := color.ParseOKLCH(r.Seed); err == nil {
			return oc
		}
	}
	return color.OKLCH{L: 0.63, C: 0.05, H: 230}
}

// qualitativeHues returns N evenly-spread OKLCH colors at a shared L/C,
// anchored at the primary hue (§5.5).
func qualitativeHues(c config.Config, n int) []color.OKLCH {
	p := primarySeed(c)
	L, chroma := 0.65, math.Max(p.C, 0.12)
	out := make([]color.OKLCH, n)
	for i := range n {
		h := math.Mod(p.H+float64(i)*360/float64(n), 360)
		out[i] = color.GamutMap(color.OKLCH{L: L, C: chroma, H: h})
	}
	return out
}

func chartsPartial(c config.Config) Partial {
	ch := c.Charts
	fb := c.Output.ColorFallbacks
	build := func(invert bool) []Var {
		var vars []Var
		quals := qualitativeHues(c, ch.QualitativeCount)
		for i, q := range quals {
			if invert {
				q = color.GamutMap(color.OKLCH{L: 1 - q.L, C: q.C, H: q.H})
			}
			name := fmt.Sprintf("--chart-qualitative-%d", i+1)
			vars = append(vars, colorVars(name, q, fb)...)
			on := color.FindOnColor(q, ch.APCA.OnLc, 0)
			vars = append(vars, colorVars(name+"-on", on, fb)...)
		}
		// Diverging: most-separated qualitative pairs through a neutral mid.
		mid := neutralSeed(c)
		if invert {
			mid = color.OKLCH{L: 1 - mid.L, C: mid.C, H: mid.H}
		}
		half := ch.QualitativeCount / 2
		toneN := ch.ToneSteps
		for d := 1; d <= ch.DivergingCount; d++ {
			a := quals[(d-1)%len(quals)]
			b := quals[(d-1+half)%len(quals)]
			if invert {
				a = color.OKLCH{L: 1 - a.L, C: a.C, H: a.H}
				b = color.OKLCH{L: 1 - b.L, C: b.C, H: b.H}
			}
			for s := 1; s <= toneN; s++ {
				t := 0.0
				if toneN > 1 {
					t = float64(s-1) / float64(toneN-1)
				}
				col := divergeInterp(a, mid, b, t)
				name := fmt.Sprintf("--chart-diverging-%d-step-%d", d, s)
				vars = append(vars, colorVars(name, col, fb)...)
				vars = append(vars, colorVars(name+"-on", color.FindOnColor(col, ch.APCA.OnLc, 0), fb)...)
				vars = append(vars, colorVars(name+"-dim", color.FindOnColor(col, ch.APCA.DimLc, 0), fb)...)
			}
		}
		return vars
	}
	return mediaPartial("stellar:charts.css", c, build)
}

// divergeInterp blends a→mid for t<0.5 and mid→b for t>=0.5 in OKLCH.
func divergeInterp(a, mid, b color.OKLCH, t float64) color.OKLCH {
	if t < 0.5 {
		return color.GamutMap(lerpLCH(a, mid, t*2))
	}
	return color.GamutMap(lerpLCH(mid, b, (t-0.5)*2))
}

func lerpLCH(a, b color.OKLCH, t float64) color.OKLCH {
	// Interpolate hue the short way around the circle.
	dh := math.Mod(b.H-a.H+540, 360) - 180
	return color.OKLCH{
		L: a.L + (b.L-a.L)*t,
		C: a.C + (b.C-a.C)*t,
		H: math.Mod(a.H+dh*t+360, 360),
	}
}

func codePartial(c config.Config) Partial {
	fb := c.Output.ColorFallbacks
	build := func(invert bool) []Var {
		n := neutralSeed(c)
		p := primarySeed(c)
		// In light mode a code surface is dark-ish; invert flips it.
		bgL := 0.18
		fgL := 0.92
		if invert {
			bgL, fgL = 0.96, 0.22
		}
		bg := color.GamutMap(color.OKLCH{L: bgL, C: math.Min(n.C, 0.02), H: n.H})
		inner := color.GamutMap(color.OKLCH{L: clamp(bgL+0.04, 0, 1), C: math.Min(n.C, 0.02), H: n.H})
		fg := color.GamutMap(color.OKLCH{L: fgL, C: math.Min(n.C, 0.02), H: n.H})
		border := color.GamutMap(color.OKLCH{L: clamp(bgL+0.12, 0, 1), C: math.Min(n.C, 0.03), H: n.H})
		borderSub := color.GamutMap(color.OKLCH{L: clamp(bgL+0.07, 0, 1), C: math.Min(n.C, 0.02), H: n.H})
		ln := color.FindOnColor(bg, c.Code.APCA.DimLc, 0)
		lnCur := color.FindOnColor(bg, c.Code.APCA.OnLc, 0)
		hl := color.GamutMap(color.OKLCH{L: clamp(bgL+0.06, 0, 1), C: math.Min(p.C, 0.04), H: p.H})
		sel := color.GamutMap(color.OKLCH{L: clamp(bgL+0.10, 0, 1), C: math.Min(p.C, 0.06), H: p.H})
		comment := color.FindOnColor(bg, c.Code.APCA.DimLc+10, 0)

		pairs := []struct {
			name string
			c    color.OKLCH
		}{
			{"--code-bg", bg}, {"--code-inner-bg", inner}, {"--code-border", border},
			{"--code-border-subtle", borderSub}, {"--code-fg", fg}, {"--code-ln", ln},
			{"--code-ln-current", lnCur}, {"--code-highlight-line", hl},
			{"--code-selection", sel}, {"--code-comment", comment},
		}
		var vars []Var
		for _, pr := range pairs {
			vars = append(vars, colorVars(pr.name, pr.c, fb)...)
		}
		return vars
	}
	return mediaPartial("stellar:code.css", c, build)
}

func namedPartial(c config.Config) Partial {
	nm := c.Named
	fb := c.Output.ColorFallbacks
	build := func(invert bool) []Var {
		var vars []Var
		for _, tok := range nm.Tokens {
			seed, err := color.ParseOKLCH(tok.Color)
			if err != nil {
				continue
			}
			for k := -nm.NegativeSteps; k <= nm.PositiveSteps; k++ {
				oc := color.OKLCH{
					L: clamp(seed.L+float64(k)*nm.TonePerStep/100, 0, 1),
					C: math.Max(0, seed.C+float64(k)*nm.ChromaPerStep/100),
					H: math.Mod(seed.H+float64(k)*nm.HuePerStep+360, 360),
				}
				if invert {
					oc.L = clamp(1-oc.L, 0, 1)
				}
				oc = color.GamutMap(oc)
				name := fmt.Sprintf("--named-%s-%d", tok.Name, k)
				vars = append(vars, colorVars(name, oc, fb)...)
				vars = append(vars, colorVars(name+"-on", color.FindOnColor(oc, c.Colors.APCA.OnLc, 0), fb)...)
				vars = append(vars, colorVars(name+"-dim", color.FindOnColor(oc, c.Colors.APCA.DimLc, 0), fb)...)
			}
		}
		return vars
	}
	return mediaPartial("stellar:named.css", c, build)
}

// mediaPartial renders both color schemes three ways so the consuming-page
// contract (§7: data-attr:data-theme="$theme") actually restyles:
//   - light in `:root, [data-theme="light"]` (the attr selector lets a
//     body-level data-theme win over the media-query dark on :root),
//   - dark under `@media (prefers-color-scheme: dark)` gated with
//     `:root:not([data-theme="light"])` so an explicit light opt-out sticks,
//   - dark again under `[data-theme="dark"]` for the explicit toggle (§5.3).
func mediaPartial(marker string, c config.Config, build func(invert bool) []Var) Partial {
	mode := c.Output.Mode
	light := build(false)
	dark := build(true)
	var b strings.Builder
	b.WriteString(selectorBlock(`:root, [data-theme="light"]`, light, mode))
	if mode == "compact" {
		b.WriteString("@media (prefers-color-scheme:dark){")
		b.WriteString(selectorBlock(`:root:not([data-theme="light"])`, dark, mode))
		b.WriteString("}")
		b.WriteString(selectorBlock(`[data-theme="dark"]`, dark, mode))
	} else {
		b.WriteString("\n@media (prefers-color-scheme: dark) {\n")
		for line := range strings.SplitSeq(selectorBlock(`:root:not([data-theme="light"])`, dark, mode), "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
		b.WriteString("}\n")
		b.WriteString(selectorBlock(`[data-theme="dark"]`, dark, mode))
	}
	return Partial{Marker: marker, Raw: b.String()}
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

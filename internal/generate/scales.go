package generate

import (
	"fmt"
	"math"

	"github.com/starfederation/stellar/internal/config"
	"github.com/starfederation/stellar/internal/scale"
)

func viewport(c config.Config) scale.Viewport {
	return scale.Viewport{Min: c.Viewport.Min, Max: c.Viewport.Max, BaseFont: c.Viewport.BaseFont}
}

func fluid(s config.SizeScale, pinZero bool) scale.FluidScale {
	return scale.FluidScale{
		BaseMin: s.BaseMin, BaseMax: s.BaseMax,
		MinRatio: s.MinRatio, MaxRatio: s.MaxRatio,
		NegativeSteps: s.NegativeSteps, PositiveSteps: s.PositiveSteps,
		PinZero: pinZero,
	}
}

// stepName builds "--prefix-n" / "--prefix--n" with the dash convention used
// in the captured output (negative steps keep a leading dash from %d, giving
// e.g. "--size--1").
func stepName(prefix string, n int) string {
	return fmt.Sprintf("--%s-%d", prefix, n)
}

func fontsSizesPartial(c config.Config) Partial {
	fs := fluid(c.FontsSizes, true)
	vp := viewport(c)
	var vars []Var
	for _, n := range fs.Steps() {
		vars = append(vars, Var{stepName("font-size", n), fs.Step(n, vp)})
	}
	return Partial{Marker: "stellar:fonts-sizes.css", Vars: vars}
}

func sizePartial(c config.Config) Partial {
	fs := fluid(c.Size, false)
	vp := viewport(c)
	var vars []Var
	// Viewport vars are emitted near the top of the size block (§3 note).
	vars = append(vars,
		Var{"--viewport-max", scale.Num(c.Viewport.Max) + "px"},
		Var{"--viewport-base-font-size", scale.Num(c.Viewport.BaseFont) + "px"},
	)
	for _, n := range fs.Steps() {
		vars = append(vars, Var{stepName("size", n), fs.Step(n, vp)})
	}
	// Pairs → "var(--size-a) var(--size-b)".
	for _, p := range c.Size.Pairs {
		vars = append(vars, Var{"--size-" + p.Name,
			fmt.Sprintf("var(--size-%d) var(--size-%d)", p.A, p.B)})
	}
	// Named aliases.
	for _, nm := range c.Size.Named {
		vars = append(vars, Var{"--size-" + nm.Name, fmt.Sprintf("var(--size-%d)", nm.Step)})
	}
	// Custom one-offs.
	for _, ct := range c.Size.Custom {
		vars = append(vars, Var{"--size-custom-" + ct.Name, ct.Value})
	}
	return Partial{Marker: "stellar:general-size.css", Vars: vars}
}

// lineHeightsPartial: flat rem values, base*ratio^n (§4.3).
func lineHeightsPartial(c config.Config) Partial {
	lh := c.FontsLineHeights
	var vars []Var
	for n := -lh.NegativeSteps; n <= lh.PositiveSteps; n++ {
		val := lh.Base * math.Pow(lh.Ratio, float64(n))
		vars = append(vars, Var{stepName("font-line-height", n), scale.Num(val) + "rem"})
	}
	return Partial{Marker: "stellar:fonts-line-heights.css", Vars: vars}
}

// letterSpacingPartial reproduces the optical-tracking shape (§4.3): tracking
// shrinks as size (step) grows, fluidly interpolated between minFactor and
// maxFactor across the viewport.
func letterSpacingPartial(c config.Config) Partial {
	sp := c.FontsSpacing
	vp := viewport(c)
	span := vp.Max - vp.Min
	inv := 0.0
	if span != 0 {
		inv = 1.0 / span
	}
	// Fluid progress t in [0,1] across the viewport.
	t := fmt.Sprintf("clamp(0, calc((100vw - %spx) * %s), 1)", scale.Num(vp.Min), scale.Num(inv))
	// factor = minFactor + (maxFactor-minFactor)*t
	factor := fmt.Sprintf("calc(%s + (%s) * %s)",
		scale.Num(sp.MinFactor), scale.Num(sp.MaxFactor-sp.MinFactor), t)
	base := scale.Num(sp.Base)

	var vars []Var
	for n := -sp.NegativeSteps; n <= sp.PositiveSteps; n++ {
		name := stepName("font-letter-spacing", n)
		var val string
		switch {
		case n == 0:
			val = "calc((max(0, " + base + ") - max(0, " + base + ")) * 1em)"
		case n > 0:
			val = fmt.Sprintf("calc(max(0, %s) * pow(max(1, %s), %d) * 1em)", base, factor, n)
		default: // n < 0 → decay, negative direction
			val = fmt.Sprintf("calc(max(0, %s) * pow(max(1, %s), %d) * -1em)", base, factor, -n)
		}
		vars = append(vars, Var{name, val})
	}
	return Partial{Marker: "stellar:fonts-spacing.css", Vars: vars}
}

package generate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/starfederation/stellar/internal/config"
	"github.com/starfederation/stellar/internal/scale"
)

func aspectRatioPartial(c config.Config) Partial {
	var vars []Var
	for _, name := range sortedKeys(c.AspectRatio) {
		vars = append(vars, Var{"--aspect-ratio-" + name, scale.Num(c.AspectRatio[name])})
	}
	return Partial{Marker: "stellar:general-aspect-ratio.css", Vars: vars}
}

func zIndexPartial(c config.Config) Partial {
	var vars []Var
	for _, z := range c.ZIndex {
		vars = append(vars, Var{"--zindex-" + z.Name, strconv.FormatInt(z.Value, 10)})
	}
	return Partial{Marker: "stellar:general-zindex.css", Vars: vars}
}

// radiusPartial: flat modular scale of border radii plus a pill token (§6).
func radiusPartial(c config.Config) Partial {
	r := c.Radius
	var vars []Var
	for n := 1; n <= r.Steps; n++ {
		val := scale.ModularValue(n-1, r.Base, r.Ratio)
		vars = append(vars, Var{fmt.Sprintf("--border-radius-%d", n), scale.Num(val) + "rem"})
	}
	vars = append(vars, Var{"--border-radius-round", "9999px"})
	return Partial{Marker: "stellar:general-radius.css", Vars: vars}
}

func borderPartial(c config.Config) Partial {
	var vars []Var
	for i, w := range c.Border.Sizes {
		vars = append(vars, Var{fmt.Sprintf("--border-size-%d", i+1), scale.Num(w) + "px"})
	}
	return Partial{Marker: "stellar:general-border.css", Vars: vars}
}

// animationPartial: duration steps base*ratio^n plus named easings (§6).
func animationPartial(c config.Config) Partial {
	a := c.Animation
	var vars []Var
	vars = append(vars, Var{"--anim-duration-base", scale.Num(a.DurationBase) + "s"})
	for n := -a.NegativeSteps; n <= a.PositiveSteps; n++ {
		val := scale.ModularValue(n, a.DurationBase, a.DurationRatio)
		vars = append(vars, Var{fmt.Sprintf("--anim-duration-%d", n), scale.Num(val) + "s"})
	}
	for _, name := range sortedKeys(a.Easings) {
		vars = append(vars, Var{"--anim-ease-" + name, a.Easings[name]})
	}
	return Partial{Marker: "stellar:general-animation.css", Vars: vars}
}

// gradientsPartial derives per-role and custom gradients from the theme
// ramps (§6). Values are var() references, so gradients restyle with the
// ramps and need no light/dark duplication.
func gradientsPartial(c config.Config) Partial {
	g := c.Gradients
	stops := func(from, to string) string {
		return fmt.Sprintf("linear-gradient(%sdeg in %s, var(--%s), var(--%s))",
			scale.Num(g.Angle), g.Space, from, to)
	}
	var vars []Var
	for _, role := range roleOrder(c.Colors.Roles) {
		vars = append(vars, Var{"--gradient-" + role,
			stops(fmt.Sprintf("%s-%d", role, g.From), fmt.Sprintf("%s-%d", role, g.To))})
	}
	for _, p := range g.Pairs {
		vars = append(vars, Var{"--gradient-" + p.Name, stops(p.From, p.To)})
	}
	return Partial{Marker: "stellar:gradients.css", Vars: vars}
}

func fontsFamiliesPartial(c config.Config) Partial {
	var vars []Var
	for _, name := range sortedKeys(c.FontsFamilies) {
		stack := c.FontsFamilies[name]
		quoted := make([]string, len(stack))
		for i, f := range stack {
			// Quote multi-word family names.
			if strings.ContainsAny(f, " ") && !strings.HasPrefix(f, "\"") {
				quoted[i] = "\"" + f + "\""
			} else {
				quoted[i] = f
			}
		}
		vars = append(vars, Var{"--font-" + name, strings.Join(quoted, ", ")})
	}
	return Partial{Marker: "stellar:fonts-families.css", Vars: vars}
}

// normalizePartial is a fixed modern reset (§6). Emitted as raw CSS rather
// than custom properties.
func normalizePartial() Partial {
	const reset = `*, *::before, *::after { box-sizing: border-box; }
* { margin: 0; padding: 0; }
html { -webkit-text-size-adjust: 100%; text-size-adjust: 100%; }
body { min-height: 100vh; line-height: 1.5; -webkit-font-smoothing: antialiased; }
img, picture, video, canvas, svg { display: block; max-width: 100%; }
input, button, textarea, select { font: inherit; }
p, h1, h2, h3, h4, h5, h6 { overflow-wrap: break-word; }`
	return Partial{Marker: "stellar:general-normalize.css", Raw: reset}
}

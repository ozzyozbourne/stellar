package generate

import (
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

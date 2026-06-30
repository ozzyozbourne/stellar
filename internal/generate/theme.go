package generate

import (
	"fmt"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
)

// colorVars emits one custom property, optionally preceded by a hex-fallback
// declaration of the same name (§9 output.colorFallbacks). CSS keeps the last
// value it parses, so the #rrggbb fallback is listed first and oklch() second.
func colorVars(name string, c color.OKLCH, fallbacks bool) []Var {
	if fallbacks {
		return []Var{{name, c.HexFallback()}, {name, c.CSS()}}
	}
	return []Var{{name, c.CSS()}}
}

// roleVars emits --{role}-{n}, -on, -dim for one role's ramp.
func roleVars(role string, tones []color.Tone, fallbacks bool) []Var {
	var vars []Var
	for _, t := range tones {
		base := fmt.Sprintf("--%s-%d", role, t.Step)
		vars = append(vars, colorVars(base, t.Bg, fallbacks)...)
		vars = append(vars, colorVars(base+"-on", t.On, fallbacks)...)
		vars = append(vars, colorVars(base+"-dim", t.Dim, fallbacks)...)
	}
	return vars
}

func ramp(seed config.Role, colors config.Colors, invert bool) []color.Tone {
	oc, err := color.ParseOKLCH(seed.Seed)
	if err != nil {
		oc = color.OKLCH{L: 0.63, C: 0.05, H: 230}
	}
	return color.Ramp(oc, color.RampOpts{
		Steps:  colors.Steps,
		OnLc:   colors.APCA.OnLc,
		DimLc:  colors.APCA.DimLc,
		Invert: invert,
	})
}

// roleOrder keeps primary/secondary/neutral first, then any extras sorted.
func roleOrder(roles map[string]config.Role) []string {
	pref := []string{"primary", "secondary", "neutral"}
	seen := map[string]bool{}
	var out []string
	for _, p := range pref {
		if _, ok := roles[p]; ok {
			out = append(out, p)
			seen[p] = true
		}
	}
	for _, k := range sortedKeys(roles) {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

func colorsThemePartial(c config.Config) Partial {
	fb := c.Output.ColorFallbacks
	order := roleOrder(c.Colors.Roles)
	// invertLightDark (§5.3) swaps which palette drives light vs dark.
	build := func(invert bool) []Var {
		useInvert := invert != c.Colors.Schemes.InvertLightDark
		var vars []Var
		for _, role := range order {
			vars = append(vars, roleVars(role, ramp(c.Colors.Roles[role], c.Colors, useInvert), fb)...)
		}
		return vars
	}
	return mediaPartial("stellar:colors-theme.css", c, build)
}

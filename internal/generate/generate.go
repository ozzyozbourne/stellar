// Package generate assembles stellar.css from a config: one emitter per
// token family, concatenated as marked partials (stellar.md §3, §6).
package generate

import (
	"sort"
	"strings"
	"fmt"

	"github.com/starfederation/stellar/internal/config"
)

// Var is a single custom property.
type Var struct{ Name, Value string }

// Partial is one labelled output block.
type Partial struct {
	Marker string // e.g. "stellar:general-size.css"
	// Raw, when non-empty, is emitted verbatim after the marker (used for
	// blocks that need @media or multiple :root rules). Otherwise Vars are
	// wrapped in a single :root{}.
	Vars []Var
	Raw  string
}

// CSS renders the partial in the given output mode ("readable"|"compact").
func (p Partial) CSS(mode string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/* %s */", p.Marker)
	// Marker-only partials (e.g. fonts-import) emit no body.
	if p.Raw == "" && len(p.Vars) == 0 {
		return b.String()
	}
	if mode != "compact" {
		b.WriteByte('\n')
	}
	if p.Raw != "" {
		b.WriteString(p.Raw)
		return b.String()
	}
	b.WriteString(rootBlock(p.Vars, mode))
	return b.String()
}

// rootBlock wraps vars in a :root{} rule.
func rootBlock(vars []Var, mode string) string {
	var b strings.Builder
	if mode == "compact" {
		b.WriteString(":root{")
		for _, v := range vars {
			fmt.Fprintf(&b, "%s:%s;", v.Name, v.Value)
		}
		b.WriteString("}")
		return b.String()
	}
	b.WriteString(":root {\n")
	for _, v := range vars {
		fmt.Fprintf(&b, "  %s: %s;\n", v.Name, v.Value)
	}
	b.WriteString("}")
	return b.String()
}

// Generate produces the full stellar.css for a config (§3 order).
func Generate(c config.Config) string {
	mode := c.Output.Mode
	var out []string

	// @import (fonts) sits at the very top, before any marker.
	if c.Sections.FontsFamilies && c.FontsImport != "" {
		out = append(out, "@import \""+c.FontsImport+"\";")
	}

	type sect struct {
		on bool
		p  Partial
	}
	sects := []sect{
		{c.Sections.FontsFamilies && c.FontsImport != "", Partial{Marker: "stellar:fonts-import.css", Raw: ""}},
		{c.Sections.Normalize, normalizePartial()},
		{c.Sections.ColorsTheme, colorsThemePartial(c)},
		{c.Sections.FontsSizes, fontsSizesPartial(c)},
		{c.Sections.FontsLineHeights, lineHeightsPartial(c)},
		{c.Sections.FontsSpacing, letterSpacingPartial(c)},
		{c.Sections.Size, sizePartial(c)},
		{c.Sections.AspectRatio, aspectRatioPartial(c)},
		{c.Sections.ZIndex, zIndexPartial(c)},
		{c.Sections.Charts, chartsPartial(c)},
		{c.Sections.Code, codePartial(c)},
		{c.Sections.Named, namedPartial(c)},
		{c.Sections.FontsFamilies, fontsFamiliesPartial(c)},
	}
	for _, s := range sects {
		if !s.on {
			continue
		}
		css := s.p.CSS(mode)
		if strings.TrimSpace(css) == "" {
			continue
		}
		out = append(out, css)
	}

	sep := "\n"
	if mode != "compact" {
		sep = "\n\n"
	}
	return strings.Join(out, sep) + "\n"
}

// sortedKeys returns map keys in deterministic order.
func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

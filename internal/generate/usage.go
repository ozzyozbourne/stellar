// Token usage analysis: report how a site's hand-written sources use the
// generated custom properties. Report-only by design — purging a ~10 KB
// cached, gzip-friendly stylesheet is not worth breaking the "tokens are
// always there when you need them" contract.
package generate

import (
	"regexp"
	"strings"
)

var declRe = regexp.MustCompile(`(--[A-Za-z0-9-]+)\s*:`)
var refRe = regexp.MustCompile(`--[A-Za-z0-9-]+`)

// TokenNames returns the custom-property names declared in generated CSS,
// deduplicated, in first-seen order.
func TokenNames(css string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range declRe.FindAllStringSubmatch(css, -1) {
		if name := m[1]; !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// FamilyUsage is the rollup for one token family (e.g. "primary" for
// --primary-3, "font-size" for --font-size--2).
type FamilyUsage struct {
	Family string
	Total  int
	Used   int
}

// Report summarizes token usage across a set of sources.
type Report struct {
	Defined  int
	Used     []string
	Unused   []string
	Families []FamilyUsage // sorted by family name
}

var stepSeg = regexp.MustCompile(`^-?\d+$`)

// family strips trailing step and variant segments and the leading dashes:
// --primary-3 → "primary", --primary-3-on → "primary",
// --font-size--2 → "font-size", --code-bg → "code-bg",
// --chart-diverging-1-step-3 → "chart-diverging".
func family(token string) string {
	name := strings.TrimPrefix(token, "--")
	segs := strings.Split(name, "-")
	for len(segs) > 1 {
		last := segs[len(segs)-1]
		// A negative step ("--2") splits into ["", "2"]; drop both parts.
		if last == "" || stepSeg.MatchString(last) ||
			last == "on" || last == "dim" || last == "step" {
			segs = segs[:len(segs)-1]
			continue
		}
		break
	}
	return strings.Join(segs, "-")
}

// Usage counts each defined token's occurrences across the source contents.
// A token counts as used when its exact name appears in any source (matches
// var(--x) references as well as bare custom-property usage); longer token
// names are not confused with their prefixes.
func Usage(defined []string, sources map[string]string) Report {
	hits := map[string]bool{}
	for _, src := range sources {
		for _, ref := range refRe.FindAllString(src, -1) {
			hits[ref] = true
		}
	}
	rep := Report{Defined: len(defined)}
	fams := map[string]*FamilyUsage{}
	for _, tok := range defined {
		f := family(tok)
		fu := fams[f]
		if fu == nil {
			fu = &FamilyUsage{Family: f}
			fams[f] = fu
		}
		fu.Total++
		if hits[tok] {
			fu.Used++
			rep.Used = append(rep.Used, tok)
		} else {
			rep.Unused = append(rep.Unused, tok)
		}
	}
	for _, f := range sortedKeys(fams) {
		rep.Families = append(rep.Families, *fams[f])
	}
	return rep
}

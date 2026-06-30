package color

import "math"

// rotate returns the seed with hue advanced by deg (preserving L, C).
func rotate(seed OKLCH, deg float64) OKLCH {
	return OKLCH{L: seed.L, C: seed.C, H: math.Mod(seed.H+deg+360, 360)}
}

// Harmony derives a set of seeds from a primary seed by hue rotation (§5.4).
// The primary is always first. Supported modes: complement, triad, tetrad
// (a.k.a. square), analogous; anything else returns just the primary.
func Harmony(seed OKLCH, mode string) []OKLCH {
	switch mode {
	case "complement":
		return []OKLCH{seed, rotate(seed, 180)}
	case "triad":
		return []OKLCH{seed, rotate(seed, 120), rotate(seed, 240)}
	case "tetrad", "square":
		return []OKLCH{seed, rotate(seed, 90), rotate(seed, 180), rotate(seed, 270)}
	case "analogous":
		return []OKLCH{seed, rotate(seed, -30), rotate(seed, 30)}
	default:
		return []OKLCH{seed}
	}
}

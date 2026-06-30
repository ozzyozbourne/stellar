// Package scale implements the parametric scale engine (stellar.md §4):
// modular scale values plus Utopia-style fluid clamp() locks. This single
// engine drives the `size`, `fontsSizes`, and (a simpler variant of)
// line-height token families.
package scale

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Viewport holds the fluid range and base font size (all in px).
type Viewport struct {
	Min      float64 // px, e.g. 320
	Max      float64 // px, e.g. 1440
	BaseFont float64 // px, e.g. 16
}

// FluidScale describes a two-lock modular scale (§4.1).
type FluidScale struct {
	BaseMin       float64
	BaseMax       float64
	MinRatio      float64
	MaxRatio      float64
	NegativeSteps int
	PositiveSteps int
	// PinZero, when true, forces step 0 to a flat 1rem regardless of base
	// (used by fontsSizes where step 0 is pinned to 1rem).
	PinZero bool
}

// ModularValue returns base * ratio^n (§4.1).
func ModularValue(n int, base, ratio float64) float64 {
	return base * math.Pow(ratio, float64(n))
}

// MinVal / MaxVal are the rem values at the min/max viewport locks.
func (s FluidScale) MinVal(n int) float64 { return ModularValue(n, s.BaseMin, s.MinRatio) }
func (s FluidScale) MaxVal(n int) float64 { return ModularValue(n, s.BaseMax, s.MaxRatio) }

// Clamp builds the CSS value for one step given the viewport (§4.2). If
// minVal == maxVal it emits a flat rem instead of a clamp().
func Clamp(minVal, maxVal float64, vp Viewport) string {
	if eq(minVal, maxVal) {
		return Num(minVal) + "rem"
	}
	bf := vp.BaseFont
	minPx := minVal * bf
	maxPx := maxVal * bf
	slope := (maxPx - minPx) / (vp.Max - vp.Min)
	interceptRem := (minPx - slope*vp.Min) / bf
	vwCoeff := slope * 100
	return fmt.Sprintf("clamp(%srem, calc(%srem + %svw), %srem)",
		Num(minVal), Num(interceptRem), Num(vwCoeff), Num(maxVal))
}

// Step returns the CSS clamp() (or flat rem) for a given step index.
func (s FluidScale) Step(n int, vp Viewport) string {
	if s.PinZero && n == 0 {
		return "1rem"
	}
	return Clamp(s.MinVal(n), s.MaxVal(n), vp)
}

// Steps yields the ordered step indices from -negative .. +positive.
func (s FluidScale) Steps() []int {
	out := make([]int, 0, s.NegativeSteps+s.PositiveSteps+1)
	for n := -s.NegativeSteps; n <= s.PositiveSteps; n++ {
		out = append(out, n)
	}
	return out
}

const eps = 1e-9

func eq(a, b float64) bool { return math.Abs(a-b) < eps }

// Num formats a float rounded to 6 decimals with trailing zeros trimmed
// (§4.2: "Round emitted numbers to 6 decimals").
func Num(f float64) string {
	// Guard against negative zero.
	if f == 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'f', 6, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

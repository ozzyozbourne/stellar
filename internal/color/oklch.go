// Package color implements the OKLCH color math, sRGB gamut mapping, APCA-W3
// contrast, and ramp generation used by the theme/charts/code/named emitters
// (stellar.md §5).
package color

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGB is linear-free, gamma-encoded sRGB in [0,1].
type RGB struct{ R, G, B float64 }

// OKLab is a perceptual Lab. L in [0,1].
type OKLab struct{ L, A, B float64 }

// OKLCH is OKLab in polar form. L in [0,1], C chroma, H degrees.
type OKLCH struct{ L, C, H float64 }

func (c OKLCH) ToLab() OKLab {
	h := c.H * math.Pi / 180
	return OKLab{L: c.L, A: c.C * math.Cos(h), B: c.C * math.Sin(h)}
}

func (l OKLab) ToLCH() OKLCH {
	c := math.Hypot(l.A, l.B)
	h := math.Atan2(l.B, l.A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return OKLCH{L: l.L, C: c, H: h}
}

// --- sRGB gamma transfer ---

func linToGamma(x float64) float64 {
	if x <= 0.0031308 {
		return 12.92 * x
	}
	return 1.055*math.Pow(x, 1/2.4) - 0.055
}

func gammaToLin(x float64) float64 {
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

// --- OKLab <-> linear sRGB (Ottosson) ---

func (lab OKLab) toLinearRGB() (r, g, b float64) {
	l_ := lab.L + 0.3963377774*lab.A + 0.2158037573*lab.B
	m_ := lab.L - 0.1055613458*lab.A - 0.0638541728*lab.B
	s_ := lab.L - 0.0894841775*lab.A - 1.2914855480*lab.B
	l := l_ * l_ * l_
	m := m_ * m_ * m_
	s := s_ * s_ * s_
	r = 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g = -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	b = -0.0041960863*l - 0.7034186147*m + 1.7076147010*s
	return
}

func labFromLinearRGB(r, g, b float64) OKLab {
	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b
	l_ := math.Cbrt(l)
	m_ := math.Cbrt(m)
	s_ := math.Cbrt(s)
	return OKLab{
		L: 0.2104542553*l_ + 0.7936177850*m_ - 0.0040720468*s_,
		A: 1.9779984951*l_ - 2.4285922050*m_ + 0.4505937099*s_,
		B: 0.0259040371*l_ + 0.7827717662*m_ - 0.8086757660*s_,
	}
}

// ToRGB converts OKLCH to gamma-encoded sRGB (may be out of [0,1]).
func (c OKLCH) ToRGB() RGB {
	r, g, b := c.ToLab().toLinearRGB()
	return RGB{linToGamma(r), linToGamma(g), linToGamma(b)}
}

// FromRGB converts gamma sRGB to OKLCH.
func FromRGB(c RGB) OKLCH {
	return labFromLinearRGB(gammaToLin(c.R), gammaToLin(c.G), gammaToLin(c.B)).ToLCH()
}

const gamutEps = 1e-4

func inGamut(c OKLCH) bool {
	r, g, b := c.ToLab().toLinearRGB()
	rgb := RGB{linToGamma(r), linToGamma(g), linToGamma(b)}
	return rgb.R >= -gamutEps && rgb.R <= 1+gamutEps &&
		rgb.G >= -gamutEps && rgb.G <= 1+gamutEps &&
		rgb.B >= -gamutEps && rgb.B <= 1+gamutEps
}

// GamutMap reduces chroma (preserving L and H) by binary search until the
// color fits in sRGB — the CSS Color 4 "oklch gamut map" (§5.1 step 2).
func GamutMap(c OKLCH) OKLCH {
	if c.L >= 1 {
		return OKLCH{1, 0, c.H}
	}
	if c.L <= 0 {
		return OKLCH{0, 0, c.H}
	}
	if inGamut(c) {
		return c
	}
	lo, hi := 0.0, c.C
	for range 32 {
		mid := (lo + hi) / 2
		if inGamut(OKLCH{c.L, mid, c.H}) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return OKLCH{c.L, lo, c.H}
}

// Clamped returns the in-[0,1] sRGB after gamut mapping.
func (c OKLCH) Clamped() RGB {
	rgb := GamutMap(c).ToRGB()
	return RGB{clamp01(rgb.R), clamp01(rgb.G), clamp01(rgb.B)}
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// CSS renders an OKLCH as `oklch(L% C H)` rounded to 6 dp (L as percent).
func (c OKLCH) CSS() string {
	return fmt.Sprintf("oklch(%s%% %s %s)", num(c.L*100), num(c.C), num(c.H))
}

// HexFallback renders the gamut-mapped sRGB as #rrggbb (for colorFallbacks).
func (c OKLCH) HexFallback() string {
	rgb := c.Clamped()
	return fmt.Sprintf("#%02x%02x%02x",
		int(math.Round(rgb.R*255)), int(math.Round(rgb.G*255)), int(math.Round(rgb.B*255)))
}

func num(f float64) string {
	if f == 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'f', 6, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// ParseOKLCH parses `oklch(L% C H)` or a `#rrggbb` hex into OKLCH.
func ParseOKLCH(s string) (OKLCH, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		return parseHex(s)
	}
	if !strings.HasPrefix(s, "oklch(") || !strings.HasSuffix(s, ")") {
		return OKLCH{}, fmt.Errorf("not an oklch()/hex color: %q", s)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(s, "oklch("), ")")
	fields := strings.Fields(body)
	if len(fields) != 3 {
		return OKLCH{}, fmt.Errorf("oklch needs 3 components: %q", s)
	}
	l, err := parseComponent(fields[0], true)
	if err != nil {
		return OKLCH{}, err
	}
	c, err := parseComponent(fields[1], false)
	if err != nil {
		return OKLCH{}, err
	}
	h, err := parseComponent(fields[2], false)
	if err != nil {
		return OKLCH{}, err
	}
	return OKLCH{L: l, C: c, H: h}, nil
}

func parseComponent(s string, pct bool) (float64, error) {
	if before, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(before, 64)
		return v / 100, err
	}
	v, err := strconv.ParseFloat(s, 64)
	if pct {
		return v, err // L given as 0..1
	}
	return v, err
}

func parseHex(s string) (OKLCH, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return OKLCH{}, fmt.Errorf("bad hex: #%s", s)
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return OKLCH{}, err
	}
	r := float64((n>>16)&0xff) / 255
	g := float64((n>>8)&0xff) / 255
	b := float64(n&0xff) / 255
	return FromRGB(RGB{r, g, b}), nil
}

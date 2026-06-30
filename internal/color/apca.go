package color

import "math"

// APCA-W3 constants (SA98G, the published 0.1.9 lookup) — stellar.md §5.2, §12.
// Frozen 4g constants (since Feb 15 2021):
//   https://github.com/Myndex/SAPC-APCA/blob/master/documentation/ImportantChangeNotices.md
// Full formula with named variables (LaTeX spec):
//   https://github.com/Myndex/SAPC-APCA/blob/master/documentation/APCA-W3-LaTeX.md
// Official W3 reference implementation (v0.1.9):
//   https://github.com/Myndex/apca-w3
const (
	apcaTRC = 2.4
	// sRGB→CIE XYZ luminance coefficients (Y row, D65 white point, high-precision matrix):
	// https://www.brucelindbloom.com/index.html?Eqn_RGB_XYZ_Matrix.html
	sRco = 0.2126729
	sGco = 0.7151522
	sBco = 0.0721750

	normBG  = 0.56
	normTXT = 0.57
	revTXT  = 0.62
	revBG   = 0.65

	blkThrs     = 0.022
	blkClmp     = 1.414
	scaleBoW    = 1.14
	scaleWoB    = 1.14
	loBoWoffset = 0.027
	loWoBoffset = 0.027
	deltaYmin   = 0.0005
	loClip      = 0.1
)

// LumaY computes APCA screen luminance Y from gamma sRGB (simple 2.4 power,
// not the piecewise sRGB transfer — APCA uses the estimated-display TRC).
func (c RGB) LumaY() float64 {
	return sRco*math.Pow(c.R, apcaTRC) +
		sGco*math.Pow(c.G, apcaTRC) +
		sBco*math.Pow(c.B, apcaTRC)
}

func softClampBlack(y float64) float64 {
	if y > blkThrs {
		return y
	}
	return y + math.Pow(blkThrs-y, blkClmp)
}

// APCAContrast returns Lc (lightness contrast) for text luminance over
// background luminance. Positive ≈ dark-on-light, negative ≈ light-on-dark.
func APCAContrast(txtY, bgY float64) float64 {
	txtY = softClampBlack(txtY)
	bgY = softClampBlack(bgY)
	if math.Abs(bgY-txtY) < deltaYmin {
		return 0
	}
	var out float64
	if bgY > txtY {
		sapc := (math.Pow(bgY, normBG) - math.Pow(txtY, normTXT)) * scaleBoW
		if sapc < loClip {
			return 0
		}
		out = sapc - loBoWoffset
	} else {
		sapc := (math.Pow(bgY, revBG) - math.Pow(txtY, revTXT)) * scaleWoB
		if sapc > -loClip {
			return 0
		}
		out = sapc + loWoBoffset
	}
	return out * 100
}

// APCABetween returns |Lc| between two OKLCH colors (after gamut mapping).
func APCABetween(text, bg OKLCH) float64 {
	return math.Abs(APCAContrast(text.Clamped().LumaY(), bg.Clamped().LumaY()))
}

// FindOnColor searches lightness for a foreground hitting targetLc against the
// given background, preserving hue at a low chroma (§5.2). Among lightness
// candidates that meet-or-exceed the target it picks the smallest overshoot
// (keeps "on" colors close to target); if none reach it, it returns the
// maximum-contrast candidate.
func FindOnColor(bg OKLCH, targetLc, chroma float64) OKLCH {
	bgY := bg.Clamped().LumaY()
	const steps = 200

	meet := OKLCH{}
	meetOver := math.Inf(1)
	best := OKLCH{L: 0, C: 0, H: bg.H}
	bestLc := -1.0

	for i := 0; i <= steps; i++ {
		L := float64(i) / steps
		cand := GamutMap(OKLCH{L: L, C: chroma, H: bg.H})
		lc := math.Abs(APCAContrast(cand.Clamped().LumaY(), bgY))
		if lc > bestLc {
			bestLc = lc
			best = cand
		}
		if lc >= targetLc && lc-targetLc < meetOver {
			meetOver = lc - targetLc
			meet = cand
		}
	}
	if meetOver != math.Inf(1) {
		return meet
	}
	return best
}

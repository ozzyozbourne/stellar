package color

import (
	"math"
	"testing"
)

// APCA reference pairs (apca-w3 0.1.9). Black-on-white ≈ 106.04,
// white-on-black ≈ -107.88.
func TestAPCAReference(t *testing.T) {
	black := RGB{0, 0, 0}.LumaY()
	white := RGB{1, 1, 1}.LumaY()

	if lc := APCAContrast(black, white); math.Abs(lc-106.04) > 0.2 {
		t.Errorf("black on white Lc = %.2f, want ~106.04", lc)
	}
	if lc := APCAContrast(white, black); math.Abs(lc-(-107.88)) > 0.2 {
		t.Errorf("white on black Lc = %.2f, want ~-107.88", lc)
	}
}

func TestAPCAMidGrey(t *testing.T) {
	// #888 on white — known APCA ≈ 63.1 (sanity band).
	g := RGB{0x88 / 255.0, 0x88 / 255.0, 0x88 / 255.0}.LumaY()
	w := RGB{1, 1, 1}.LumaY()
	lc := APCAContrast(g, w)
	if lc < 55 || lc > 70 {
		t.Errorf("#888 on white Lc = %.2f, want 55..70", lc)
	}
}

func TestRoundTrip(t *testing.T) {
	// OKLCH -> RGB -> OKLCH should be stable for in-gamut colors.
	in := OKLCH{L: 0.63, C: 0.05, H: 230}
	out := FromRGB(in.ToRGB())
	if math.Abs(in.L-out.L) > 1e-3 || math.Abs(in.C-out.C) > 1e-3 {
		t.Errorf("round trip drift: in %v out %v", in, out)
	}
}

func TestGamutMapInGamut(t *testing.T) {
	// A wildly out-of-gamut chroma must be pulled back into sRGB.
	c := GamutMap(OKLCH{L: 0.7, C: 0.5, H: 30})
	rgb := c.ToRGB()
	for _, v := range []float64{rgb.R, rgb.G, rgb.B} {
		if v < -1e-3 || v > 1+1e-3 {
			t.Errorf("gamut map left channel out of range: %v", rgb)
		}
	}
}

func TestFindOnColorMeetsTarget(t *testing.T) {
	bg := OKLCH{L: 0.95, C: 0.02, H: 230} // light surface
	on := FindOnColor(bg, 90, 0)
	if got := APCABetween(on, bg); got < 90 {
		t.Errorf("on color Lc %.1f < 90 target", got)
	}
}

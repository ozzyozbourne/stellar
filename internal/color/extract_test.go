package color

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// makeImage paints vertical bands of the given RGBA colors.
func makeImage(w, h int, cols []color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	band := w / len(cols)
	for x := range w {
		c := cols[imin(x/band, len(cols)-1)]
		for y := range h {
			img.Set(x, y, c)
		}
	}
	return img
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestExtractFindsDominantColors(t *testing.T) {
	red := color.RGBA{220, 30, 30, 255}
	blue := color.RGBA{30, 60, 210, 255}
	green := color.RGBA{40, 180, 60, 255}
	img := makeImage(300, 100, []color.RGBA{red, blue, green})

	cands := Extract(img, 3)
	if len(cands) < 3 {
		t.Fatalf("expected >=3 candidates, got %d", len(cands))
	}
	// Each input hue should appear among the candidates.
	want := map[string]OKLCH{
		"red":   FromRGB(RGB{220.0 / 255, 30.0 / 255, 30.0 / 255}),
		"blue":  FromRGB(RGB{30.0 / 255, 60.0 / 255, 210.0 / 255}),
		"green": FromRGB(RGB{40.0 / 255, 180.0 / 255, 60.0 / 255}),
	}
	for label, target := range want {
		matched := false
		for _, c := range cands {
			dh := math.Abs(math.Mod(c.Color.H-target.H+540, 360) - 180)
			if dh < 20 {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("no candidate near %s (hue %.0f); got %+v", label, target.H, hues(cands))
		}
	}
	// Populations sum to ~1 and stars are within range.
	var sum float64
	for _, c := range cands {
		sum += c.Population
		if c.Stars < 1 || c.Stars > 5 {
			t.Errorf("stars out of range: %d", c.Stars)
		}
	}
	if math.Abs(sum-1) > 0.05 {
		t.Errorf("populations sum %.3f, want ~1", sum)
	}
}

func hues(cs []Candidate) []float64 {
	out := make([]float64, len(cs))
	for i, c := range cs {
		out[i] = math.Round(c.Color.H)
	}
	return out
}

func TestHarmony(t *testing.T) {
	seed := OKLCH{L: 0.63, C: 0.1, H: 30}
	comp := Harmony(seed, "complement")
	if len(comp) != 2 || math.Abs(comp[1].H-210) > 1e-6 {
		t.Errorf("complement hue = %v, want 210", comp)
	}
	tri := Harmony(seed, "triad")
	if len(tri) != 3 || math.Abs(tri[1].H-150) > 1e-6 || math.Abs(tri[2].H-270) > 1e-6 {
		t.Errorf("triad hues = %v, want 150 & 270", tri)
	}
	tet := Harmony(seed, "tetrad")
	if len(tet) != 4 {
		t.Errorf("tetrad len = %d, want 4", len(tet))
	}
	ana := Harmony(seed, "analogous")
	if len(ana) != 3 || math.Abs(ana[1].H-0) > 1e-6 || math.Abs(ana[2].H-60) > 1e-6 {
		t.Errorf("analogous hues = %v, want 0 & 60", ana)
	}
	// hue rotation wraps the circle.
	if got := rotate(OKLCH{H: 350}, 30).H; math.Abs(got-20) > 1e-6 {
		t.Errorf("rotate wrap = %.3f, want 20", got)
	}
}

func TestNameColor(t *testing.T) {
	cases := []struct {
		c    OKLCH
		want string
	}{
		{OKLCH{L: 0.95, C: 0.001, H: 0}, "white"},
		{OKLCH{L: 0.05, C: 0.001, H: 0}, "black"},
		{OKLCH{L: 0.5, C: 0.005, H: 0}, "grey"},
	}
	for _, c := range cases {
		if got := NameColor(c.c); got != c.want {
			t.Errorf("NameColor(%v) = %q, want %q", c.c, got, c.want)
		}
	}
}

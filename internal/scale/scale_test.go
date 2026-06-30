package scale

import "testing"

var vp = Viewport{Min: 320, Max: 1440, BaseFont: 16}

// font scale: baseMin=baseMax=1, minRatio 1.125, maxRatio 1.2, step 0 pinned.
var fontScale = FluidScale{BaseMin: 1, BaseMax: 1, MinRatio: 1.125, MaxRatio: 1.2,
	NegativeSteps: 2, PositiveSteps: 12, PinZero: true}

// size scale: baseMin 0.5, baseMax 1.1, minRatio 1.067, maxRatio 1.5.
var sizeScale = FluidScale{BaseMin: 0.5, BaseMax: 1.1, MinRatio: 1.067, MaxRatio: 1.5,
	NegativeSteps: 2, PositiveSteps: 12}

func TestFontSize1(t *testing.T) {
	// §4.2 verified exact clamp.
	want := "clamp(1.125rem, calc(1.103571rem + 0.107143vw), 1.2rem)"
	if got := fontScale.Step(1, vp); got != want {
		t.Errorf("font-size-1\n got: %s\nwant: %s", got, want)
	}
}

func TestFontSize0Flat(t *testing.T) {
	if got := fontScale.Step(0, vp); got != "1rem" {
		t.Errorf("font-size-0 = %q, want 1rem", got)
	}
}

func TestFontSize5Locks(t *testing.T) {
	// §4.1: clamp(1.125^5, …, 1.2^5) = clamp(1.802032rem, …, 2.48832rem).
	if got := Num(fontScale.MinVal(5)); got != "1.802032" {
		t.Errorf("font-size-5 min = %s, want 1.802032", got)
	}
	if got := Num(fontScale.MaxVal(5)); got != "2.48832" {
		t.Errorf("font-size-5 max = %s, want 2.48832", got)
	}
}

func TestSizeLocks(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"size-1 min", Num(sizeScale.MinVal(1)), "0.5335"},
		{"size-1 max", Num(sizeScale.MaxVal(1)), "1.65"},
		{"size-2 max", Num(sizeScale.MaxVal(2)), "2.475"},
		{"size--1 min", Num(sizeScale.MinVal(-1)), "0.468604"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %s, want %s", c.name, c.got, c.want)
		}
	}
}

func TestNum(t *testing.T) {
	cases := map[float64]string{
		1.2: "1.2", 1.65: "1.65", 0: "0", 0.5335: "0.5335",
		1.103571428: "1.103571",
	}
	for in, want := range cases {
		if got := Num(in); got != want {
			t.Errorf("Num(%v) = %s, want %s", in, got, want)
		}
	}
}

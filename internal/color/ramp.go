package color

// Tone is one step of a role ramp: a background plus its APCA-solved
// foreground variants.
type Tone struct {
	Step int
	Bg   OKLCH
	On   OKLCH // foreground meeting onLc
	Dim  OKLCH // muted foreground meeting dimLc
}

// RampOpts controls ramp generation (§5.1, §5.2).
type RampOpts struct {
	Steps   int
	OnLc    float64
	DimLc   float64
	Lmax    float64 // lightness of step 1 (default 0.96)
	Lmin    float64 // lightness of step N (default 0.18)
	Invert  bool    // tone-inverted dark ramp (§5.3)
	OnChrom float64 // chroma for on/dim foregrounds (default 0)
}

func (o RampOpts) withDefaults() RampOpts {
	if o.Steps == 0 {
		o.Steps = 12
	}
	if o.Lmax == 0 {
		o.Lmax = 0.96
	}
	if o.Lmin == 0 {
		o.Lmin = 0.18
	}
	if o.OnLc == 0 {
		o.OnLc = 90
	}
	if o.DimLc == 0 {
		o.DimLc = 30
	}
	return o
}

// smoothstep ease-in/out for a perceptual lightness curve.
func smoothstep(t float64) float64 { return t * t * (3 - 2*t) }

// Ramp builds the tones for a role seed (§5.1). Lightness walks an eased
// curve from Lmax (step 1) to Lmin (step N), hue is held, chroma is gamut
// mapped per step. With Invert, the lightness curve is reversed (dark theme).
func Ramp(seed OKLCH, o RampOpts) []Tone {
	o = o.withDefaults()
	n := o.Steps
	tones := make([]Tone, 0, n)
	for i := 1; i <= n; i++ {
		var t float64
		if n > 1 {
			t = float64(i-1) / float64(n-1)
		}
		if o.Invert {
			t = 1 - t
		}
		L := o.Lmax + (o.Lmin-o.Lmax)*smoothstep(t)
		bg := GamutMap(OKLCH{L: L, C: seed.C, H: seed.H})
		tones = append(tones, Tone{
			Step: i,
			Bg:   bg,
			On:   FindOnColor(bg, o.OnLc, o.OnChrom),
			Dim:  FindOnColor(bg, o.DimLc, o.OnChrom),
		})
	}
	return tones
}

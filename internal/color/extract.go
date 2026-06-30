package color

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"sort"
)

// DecodeImage decodes a PNG/JPEG/GIF from a reader.
func DecodeImage(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}

// DecodeFile decodes an image file by path.
func DecodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DecodeImage(f)
}

// Candidate is an extracted palette color with provenance (stellar.md §5.4).
type Candidate struct {
	Color      OKLCH   `json:"color"`      // gamut-mapped centroid
	CSS        string  `json:"css"`        // oklch(...) string
	Hex        string  `json:"hex"`        // #rrggbb
	Name       string  `json:"name"`       // human hue/tone name
	Population float64 `json:"population"` // fraction of sampled pixels [0,1]
	Score      float64 `json:"score"`      // population × chroma
	Stars      int     `json:"stars"`      // 1..5 star rating
}

// Extract downscales an image, clusters its pixels in OKLab (k-means), and
// scores clusters by population × chroma, returning candidates sorted best
// first (§5.4).
func Extract(img image.Image, k int) []Candidate {
	if k < 1 {
		k = 6
	}
	pts := samplePixels(img, 4096)
	if len(pts) == 0 {
		return nil
	}
	if k > len(pts) {
		k = len(pts)
	}
	centroids, counts := kmeans(pts, k, 16)

	total := float64(len(pts))
	cands := make([]Candidate, 0, k)
	for i, c := range centroids {
		if counts[i] == 0 {
			continue
		}
		lch := GamutMap(c.ToLCH())
		pop := float64(counts[i]) / total
		score := pop * lch.C
		cands = append(cands, Candidate{
			Color:      lch,
			CSS:        lch.CSS(),
			Hex:        lch.HexFallback(),
			Name:       NameColor(lch),
			Population: pop,
			Score:      score,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	rateStars(cands)
	return cands
}

// samplePixels reads up to max pixels (evenly strided) as OKLab points.
func samplePixels(img image.Image, max int) []OKLab {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}
	stride := 1
	if w*h > max {
		stride = int(math.Sqrt(float64(w*h) / float64(max)))
		if stride < 1 {
			stride = 1
		}
	}
	var pts []OKLab
	for y := b.Min.Y; y < b.Max.Y; y += stride {
		for x := b.Min.X; x < b.Max.X; x += stride {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0x8000 { // skip mostly-transparent pixels
				continue
			}
			rgb := RGB{float64(r) / 65535, float64(g) / 65535, float64(bl) / 65535}
			pts = append(pts, labFromLinearRGB(gammaToLin(rgb.R), gammaToLin(rgb.G), gammaToLin(rgb.B)))
		}
	}
	return pts
}

func labDist2(a, b OKLab) float64 {
	dl := a.L - b.L
	da := a.A - b.A
	db := a.B - b.B
	return dl*dl + da*da + db*db
}

// kmeans clusters lab points into k groups, returning centroids and sizes.
func kmeans(pts []OKLab, k, iters int) ([]OKLab, []int) {
	// Deterministic k-means++-ish init: spread initial centroids by distance.
	centroids := make([]OKLab, 0, k)
	centroids = append(centroids, pts[0])
	for len(centroids) < k {
		var far OKLab
		best := -1.0
		for _, p := range pts {
			d := math.Inf(1)
			for _, c := range centroids {
				if dd := labDist2(p, c); dd < d {
					d = dd
				}
			}
			if d > best {
				best = d
				far = p
			}
		}
		centroids = append(centroids, far)
	}

	assign := make([]int, len(pts))
	counts := make([]int, k)
	for it := range iters {
		moved := false
		for i, p := range pts {
			bestC, bestD := 0, math.Inf(1)
			for ci, c := range centroids {
				if d := labDist2(p, c); d < bestD {
					bestD, bestC = d, ci
				}
			}
			if assign[i] != bestC {
				moved = true
			}
			assign[i] = bestC
		}
		// recompute centroids
		sums := make([]OKLab, k)
		for i := range counts {
			counts[i] = 0
		}
		for i, p := range pts {
			c := assign[i]
			sums[c].L += p.L
			sums[c].A += p.A
			sums[c].B += p.B
			counts[c]++
		}
		for ci := range centroids {
			if counts[ci] > 0 {
				n := float64(counts[ci])
				centroids[ci] = OKLab{sums[ci].L / n, sums[ci].A / n, sums[ci].B / n}
			}
		}
		if !moved && it > 0 {
			break
		}
	}
	return centroids, counts
}

// rateStars assigns 1..5 stars by score quantiles (best gets 5).
func rateStars(cands []Candidate) {
	if len(cands) == 0 {
		return
	}
	max := cands[0].Score
	for _, c := range cands {
		if c.Score > max {
			max = c.Score
		}
	}
	for i := range cands {
		if max <= 0 {
			cands[i].Stars = 1
			continue
		}
		s := int(math.Ceil(5 * cands[i].Score / max))
		if s < 1 {
			s = 1
		}
		cands[i].Stars = s
	}
}

// hueName buckets map an OKLCH hue (degrees) to a name. Boundaries are tuned
// for OKLCH, where e.g. pure sRGB red sits near 29° and yellow near 100°.
var hueBuckets = []struct {
	max  float64
	name string
}{
	{35, "red"}, {70, "orange"}, {110, "yellow"}, {165, "green"},
	{210, "teal"}, {270, "blue"}, {320, "violet"},
	{350, "magenta"}, {360, "red"},
}

// NameColor produces a human name like "vivid blue" / "dark grey" (§5.4).
func NameColor(c OKLCH) string {
	tone := ""
	switch {
	case c.L >= 0.85:
		tone = "light "
	case c.L <= 0.30:
		tone = "dark "
	}
	if c.C < 0.02 {
		switch {
		case c.L >= 0.9:
			return "white"
		case c.L <= 0.12:
			return "black"
		default:
			return tone + "grey"
		}
	}
	vivid := ""
	if c.C >= 0.12 {
		vivid = "vivid "
	}
	h := math.Mod(c.H+360, 360)
	name := "red"
	for _, b := range hueBuckets {
		if h < b.max {
			name = b.name
			break
		}
	}
	return tone + vivid + name
}

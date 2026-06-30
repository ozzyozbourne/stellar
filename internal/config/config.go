// Package config defines the Stellar JSON schema (stellar.md §2), plus
// load/save and defaults. The editor is a front-end over this struct; the
// generator consumes it.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/starfederation/stellar/internal/color"
)

type Viewport struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	BaseFont float64 `json:"baseFont"`
}

// Sections toggles each output block (§2 "sections").
type Sections struct {
	Normalize        bool `json:"normalize"`
	ColorsTheme      bool `json:"colorsTheme"`
	FontsSizes       bool `json:"fontsSizes"`
	FontsLineHeights bool `json:"fontsLineHeights"`
	FontsSpacing     bool `json:"fontsSpacing"`
	FontsFamilies    bool `json:"fontsFamilies"`
	Size             bool `json:"size"`
	AspectRatio      bool `json:"aspectRatio"`
	ZIndex           bool `json:"zIndex"`
	Charts           bool `json:"charts"`
	Code             bool `json:"code"`
	Named            bool `json:"named"`
}

type Pair struct {
	Name string `json:"name"`
	A    int    `json:"a"`
	B    int    `json:"b"`
}

type NamedStep struct {
	Name string `json:"name"`
	Step int    `json:"step"`
}

// SizeScale is the fluid modular scale config (§2 "size", "fontsSizes").
type SizeScale struct {
	BaseMin       float64     `json:"baseMin"`
	BaseMax       float64     `json:"baseMax"`
	MinRatio      float64     `json:"minRatio"`
	MaxRatio      float64     `json:"maxRatio"`
	NegativeSteps int         `json:"negativeSteps"`
	PositiveSteps int         `json:"positiveSteps"`
	Pairs         []Pair      `json:"pairs,omitempty"`
	Named         []NamedStep `json:"named,omitempty"`
	// Custom one-off tokens → --size-custom-NNN (§4.3).
	Custom []CustomToken `json:"custom,omitempty"`
}

type CustomToken struct {
	Name  string `json:"name"`  // e.g. "001"
	Value string `json:"value"` // raw CSS value
}

type LineHeights struct {
	Base          float64 `json:"base"`
	NegativeSteps int     `json:"negativeSteps"`
	PositiveSteps int     `json:"positiveSteps"`
	Ratio         float64 `json:"ratio"`
}

type Spacing struct {
	Base          float64 `json:"base"`
	NegativeSteps int     `json:"negativeSteps"`
	PositiveSteps int     `json:"positiveSteps"`
	MinFactor     float64 `json:"minFactor"`
	MaxFactor     float64 `json:"maxFactor"`
}

type APCA struct {
	OnLc      float64 `json:"onLc"`
	DimLc     float64 `json:"dimLc"`
	SurfaceLc float64 `json:"surfaceLc,omitempty"`
}

type Role struct {
	Seed string `json:"seed"` // oklch(L% C H)
}

type Schemes struct {
	InvertLightDark bool `json:"invertLightDark"`
}

type Extract struct {
	Image     *string `json:"image"`
	Harmony   string  `json:"harmony"`
	Candidate int     `json:"candidate"`
}

type Colors struct {
	Space   string          `json:"space"`
	Steps   int             `json:"steps"`
	APCA    APCA            `json:"apca"`
	Schemes Schemes         `json:"schemes"`
	Roles   map[string]Role `json:"roles"`
	Extract Extract         `json:"extract"`
}

type Charts struct {
	QualitativeCount int  `json:"qualitativeCount"`
	DivergingCount   int  `json:"divergingCount"`
	ToneSteps        int  `json:"toneSteps"`
	APCA             APCA `json:"apca"`
}

type Code struct {
	APCA    APCA   `json:"apca"`
	Variant string `json:"variant"`
}

type NamedToken struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Named struct {
	NegativeSteps int          `json:"negativeSteps"`
	PositiveSteps int          `json:"positiveSteps"`
	HuePerStep    float64      `json:"huePerStep"`
	ChromaPerStep float64      `json:"chromaPerStep"`
	TonePerStep   float64      `json:"tonePerStep"`
	Tokens        []NamedToken `json:"tokens"`
}

type ZItem struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Desc  string `json:"desc,omitempty"`
}

type Output struct {
	Mode           string `json:"mode"` // readable | compact
	ColorFallbacks bool   `json:"colorFallbacks"`
}

type Config struct {
	Viewport         Viewport            `json:"viewport"`
	Sections         Sections            `json:"sections"`
	Size             SizeScale           `json:"size"`
	FontsSizes       SizeScale           `json:"fontsSizes"`
	FontsLineHeights LineHeights         `json:"fontsLineHeights"`
	FontsSpacing     Spacing             `json:"fontsSpacing"`
	FontsFamilies    map[string][]string `json:"fontsFamilies"`
	FontsImport      string              `json:"fontsImport"`
	Colors           Colors              `json:"colors"`
	Charts           Charts              `json:"charts"`
	Code             Code                `json:"code"`
	Named            Named               `json:"named"`
	AspectRatio      map[string]float64  `json:"aspectRatio"`
	ZIndex           []ZItem             `json:"zIndex"`
	Output           Output              `json:"output"`
}

// Default returns the reference config from stellar.md §2.
func Default() Config {
	return Config{
		Viewport: Viewport{Min: 320, Max: 1440, BaseFont: 16},
		Sections: Sections{
			Normalize: true, ColorsTheme: true, FontsSizes: true,
			FontsLineHeights: true, FontsSpacing: true, FontsFamilies: true,
			Size: true, AspectRatio: true, ZIndex: true,
			Charts: true, Code: true, Named: true,
		},
		Size: SizeScale{BaseMin: 0.5, BaseMax: 1.1, MinRatio: 1.067, MaxRatio: 1.5,
			NegativeSteps: 2, PositiveSteps: 12},
		FontsSizes: SizeScale{BaseMin: 1, BaseMax: 1, MinRatio: 1.125, MaxRatio: 1.2,
			NegativeSteps: 2, PositiveSteps: 12},
		FontsLineHeights: LineHeights{Base: 1.5, NegativeSteps: 3, PositiveSteps: 4, Ratio: 1.067},
		FontsSpacing:     Spacing{Base: 0.025, NegativeSteps: 2, PositiveSteps: 3, MinFactor: 1.778, MaxFactor: 2.0},
		FontsFamilies: map[string][]string{
			"humanist":   {"Seravek", "Gill Sans Nova", "Ubuntu", "Calibri", "DejaVu Sans", "source-sans-pro", "sans-serif"},
			"industrial": {"Bahnschrift", "DIN Alternate", "Franklin Gothic Medium", "Nimbus Sans Narrow", "sans-serif-condensed", "sans-serif"},
			"mono":       {"Dank Mono", "Operator Mono", "Inconsolata", "Fira Mono", "ui-monospace", "SF Mono", "Monaco", "monospace"},
		},
		FontsImport: "https://fonts.bunny.net/css?family=inter:100,900|inconsolata:400,700&display=swap",
		Colors: Colors{
			Space: "oklch", Steps: 12,
			APCA:    APCA{OnLc: 90, DimLc: 30, SurfaceLc: 30},
			Schemes: Schemes{InvertLightDark: false},
			Roles: map[string]Role{
				"primary":   {Seed: "oklch(63% 0.05 230)"},
				"secondary": {Seed: "oklch(63% 0.07 124)"},
				"neutral":   {Seed: "oklch(63% 0.01 230)"},
			},
			Extract: Extract{Image: nil, Harmony: "complement", Candidate: 0},
		},
		Charts: Charts{QualitativeCount: 12, DivergingCount: 6, ToneSteps: 12,
			APCA: APCA{OnLc: 90, DimLc: 30}},
		Code: Code{APCA: APCA{OnLc: 90, DimLc: 30, SurfaceLc: 30}, Variant: "moon-jellyfish"},
		Named: Named{NegativeSteps: 2, PositiveSteps: 2, HuePerStep: 6, ChromaPerStep: 1.5,
			TonePerStep: 4, Tokens: []NamedToken{{Name: "adobe", Color: "#ff0000"}}},
		AspectRatio: map[string]float64{"portrait": 0.75, "widescreen": 1.7778, "square": 1},
		ZIndex: []ZItem{
			{Name: "drawer", Value: 700, Desc: "Navigation drawers and shell surfaces."},
			{Name: "dialog", Value: 800}, {Name: "dropdown", Value: 900},
			{Name: "toast", Value: 950}, {Name: "tooltip", Value: 1000},
			{Name: "important", Value: 18014398509481984},
		},
		Output: Output{Mode: "readable", ColorFallbacks: false},
	}
}

// ResolveExtraction, when colors.extract.image is set, extracts a palette from
// the image and derives the primary/secondary role seeds via the configured
// harmony mode (§5.4). Neutral is set to a near-grey of the primary hue. It is
// a no-op when no image is configured.
func (c *Config) ResolveExtraction() error {
	ex := c.Colors.Extract
	if ex.Image == nil || *ex.Image == "" {
		return nil
	}
	img, err := color.DecodeFile(*ex.Image)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	cands := color.Extract(img, 8)
	if len(cands) == 0 {
		return nil
	}
	idx := ex.Candidate
	if idx < 0 || idx >= len(cands) {
		idx = 0
	}
	primary := cands[idx].Color
	seeds := color.Harmony(primary, ex.Harmony)
	if c.Colors.Roles == nil {
		c.Colors.Roles = map[string]Role{}
	}
	c.Colors.Roles["primary"] = Role{Seed: seeds[0].CSS()}
	if len(seeds) > 1 {
		c.Colors.Roles["secondary"] = Role{Seed: seeds[1].CSS()}
	}
	if len(seeds) > 2 {
		c.Colors.Roles["tertiary"] = Role{Seed: seeds[2].CSS()}
	}
	neutral := color.OKLCH{L: primary.L, C: 0.01, H: primary.H}
	c.Colors.Roles["neutral"] = Role{Seed: neutral.CSS()}
	return nil
}

// Load reads a config file, falling back to Default() if the path is empty
// or missing.
func Load(path string) (Config, error) {
	if path == "" {
		return Default(), nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	// Start from defaults so partial configs fill in.
	c := Default()
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

// Save writes the config as indented JSON.
func Save(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

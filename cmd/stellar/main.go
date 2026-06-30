// Command stellar is a local design-token compiler: it expands a JSON config
// into a single sectioned stellar.css of :root custom properties, and can
// serve a live-refreshing editor over Datastar SSE (stellar.md §9).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/starfederation/stellar/internal/color"
	"github.com/starfederation/stellar/internal/config"
	"github.com/starfederation/stellar/internal/generate"
	"github.com/starfederation/stellar/internal/minify"
	"github.com/starfederation/stellar/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	cfgPath := fs.String("config", "stellar.json", "path to config JSON")
	out := fs.String("o", "stellar.css", "output CSS path (build)")
	addr := fs.String("addr", ":7331", "listen address (serve)")
	mode := fs.String("mode", "", "output mode: readable|compact (overrides config)")
	image := fs.String("image", "", "image path (extract)")
	harmony := fs.String("harmony", "complement", "harmony mode (extract)")
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if *mode != "" {
		cfg.Output.Mode = *mode
	}
	if err := cfg.ResolveExtraction(); err != nil {
		fatal(err)
	}

	switch cmd {
	case "build":
		css := buildCSS(cfg)
		if err := os.WriteFile(*out, []byte(css), 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote %s (%d bytes, %s)\n", *out, len(css), modeLabel(cfg))
	case "export":
		// export: write CSS to stdout.
		fmt.Print(buildCSS(cfg))
	case "minify":
		// minify a user TS/JS/CSS asset (§9): stellar minify path/to/app.ts
		path := fs.Arg(0)
		if path == "" {
			fatal(fmt.Errorf("usage: stellar minify <file.ts|.js|.css> [-o out]"))
		}
		src, err := os.ReadFile(path)
		if err != nil {
			fatal(err)
		}
		res, err := minify.Asset(string(src), path)
		if err != nil {
			fatal(err)
		}
		if *out != "" && *out != "stellar.css" {
			if err := os.WriteFile(*out, []byte(res), 0o644); err != nil {
				fatal(err)
			}
			fmt.Printf("wrote %s (%d bytes)\n", *out, len(res))
		} else {
			fmt.Print(res)
		}
	case "init":
		if err := config.Save(*cfgPath, config.Default()); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote default config to %s\n", *cfgPath)
	case "extract":
		if *image == "" {
			fatal(fmt.Errorf("usage: stellar extract --image foo.png [--harmony complement|triad|tetrad|analogous]"))
		}
		img, err := color.DecodeFile(*image)
		if err != nil {
			fatal(err)
		}
		cands := color.Extract(img, 8)
		fmt.Printf("palette candidates for %s:\n", *image)
		for i, c := range cands {
			fmt.Printf("  [%d] %-16s %s  %s  pop %4.1f%%  %s\n",
				i, c.Name, c.Hex, c.CSS, c.Population*100, stars(c.Stars))
		}
		if len(cands) > 0 {
			fmt.Printf("\nharmony (%s) from best candidate:\n", *harmony)
			for i, s := range color.Harmony(cands[0].Color, *harmony) {
				role := []string{"primary", "secondary", "tertiary", "quaternary"}
				label := "extra"
				if i < len(role) {
					label = role[i]
				}
				fmt.Printf("  %-11s %s\n", label, s.CSS())
			}
		}
	case "serve":
		srv := server.New(cfg, *cfgPath)
		fmt.Printf("stellar serving on http://localhost%s\n", *addr)
		if err := srv.Listen(*addr); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

// buildCSS generates the stylesheet, running compact mode through esbuild (§9).
func buildCSS(cfg config.Config) string {
	css := generate.Generate(cfg)
	if cfg.Output.Mode == "compact" {
		min, err := minify.CSS(css)
		if err != nil {
			fatal(err)
		}
		return min
	}
	return css
}

func stars(n int) string {
	var s strings.Builder
	for i := range 5 {
		if i < n {
			s.WriteString("★")
		} else {
			s.WriteString("☆")
		}
	}
	return s.String()
}

func modeLabel(cfg config.Config) string {
	if cfg.Output.Mode == "compact" {
		return "compact/esbuild"
	}
	return "readable"
}

func usage() {
	fmt.Fprint(os.Stderr, `stellar — local design-token compiler

usage:
  stellar init   --config stellar.json            write a default config
  stellar build  --config stellar.json -o out.css [--mode readable|compact]
  stellar export --config stellar.json            print CSS to stdout
  stellar minify app.ts [-o app.min.js]           compile/minify a TS/JS/CSS asset
  stellar extract --image foo.png [--harmony complement]   extract a palette
  stellar serve  --config stellar.json --addr :7331
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

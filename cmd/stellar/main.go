// Command stellar is a local design-token compiler: it expands a JSON config
// into a single sectioned stellar.css of :root custom properties, and can
// serve a live-refreshing editor over Datastar SSE (stellar.md §9).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	case "usage":
		// usage: report how the given sources use the generated tokens.
		// Report-only by design — no purging (a cached ~10 KB sheet isn't
		// worth breaking the "tokens are always there" contract).
		if fs.NArg() == 0 {
			fatal(fmt.Errorf("usage: stellar usage <files|dirs…>"))
		}
		sources, err := collectSources(fs.Args())
		if err != nil {
			fatal(err)
		}
		printUsage(generate.Usage(generate.TokenNames(buildCSS(cfg)), sources))
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

// collectSources reads the given files, walking directories for .css/.html.
func collectSources(args []string) (map[string]string, error) {
	sources := map[string]string{}
	addFile := func(path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources[path] = string(b)
		return nil
	}
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if err := addFile(arg); err != nil {
				return nil, err
			}
			continue
		}
		err = filepath.WalkDir(arg, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".css", ".html", ".htm":
				return addFile(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return sources, nil
}

func printUsage(rep generate.Report) {
	fmt.Printf("%d tokens defined · %d used · %d unused\n\n",
		rep.Defined, len(rep.Used), len(rep.Unused))
	fmt.Println("family                     used / total")
	for _, f := range rep.Families {
		marker := ""
		if f.Used == 0 {
			marker = "  ← unused"
		}
		fmt.Printf("  %-24s %4d / %d%s\n", f.Family, f.Used, f.Total, marker)
	}
	fmt.Println("\nfully-unused families can often be dropped by disabling their section" +
		"\n(report only — the cached stylesheet is cheap; nothing is purged)")
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
  stellar usage  <files|dirs…>                    report var(--token) usage (no purging)
  stellar serve  --config stellar.json --addr :7331
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

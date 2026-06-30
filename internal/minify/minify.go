// Package minify wraps the embedded esbuild Go API so the single binary can
// minify the generated CSS (compact mode) and compile/minify users' TS/JS
// assets (stellar.md §9).
package minify

import (
	"fmt"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

func messages(ms []api.Message) string {
	parts := make([]string, len(ms))
	for i, m := range ms {
		parts[i] = m.Text
	}
	return strings.Join(parts, "; ")
}

// CSS minifies a CSS string. Custom-property names are global, so identifier
// minification is left off; whitespace and syntax are collapsed.
func CSS(src string) (string, error) {
	res := api.Transform(src, api.TransformOptions{
		Loader:           api.LoaderCSS,
		MinifyWhitespace: true,
		MinifySyntax:     true,
	})
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("css minify: %s", messages(res.Errors))
	}
	return string(res.Code), nil
}

// Asset compiles+minifies a TS/JS/CSS asset, picking the loader from the file
// extension (".ts", ".tsx", ".jsx", ".js", ".css").
func Asset(src, filename string) (string, error) {
	loader := api.LoaderJS
	switch {
	case strings.HasSuffix(filename, ".ts"):
		loader = api.LoaderTS
	case strings.HasSuffix(filename, ".tsx"):
		loader = api.LoaderTSX
	case strings.HasSuffix(filename, ".jsx"):
		loader = api.LoaderJSX
	case strings.HasSuffix(filename, ".css"):
		loader = api.LoaderCSS
	}
	res := api.Transform(src, api.TransformOptions{
		Loader:            loader,
		MinifyWhitespace:  true,
		MinifyIdentifiers: loader != api.LoaderCSS,
		MinifySyntax:      true,
	})
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("minify %s: %s", filename, messages(res.Errors))
	}
	return string(res.Code), nil
}

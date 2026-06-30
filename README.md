# Stellar

A local **design-token compiler** with a live visual editor. It expands a small
JSON config into a single `stellar.css` of `:root` custom properties — every
value derived parametrically — and hot-swaps the stylesheet over a Datastar SSE
stream on every edit. You then write semantic, natively-nested CSS that only
references `var(--token)`.

This is a self-contained Go implementation of the spec in
[`../stellar.md`](../stellar.md). No external dependencies: OKLCH↔sRGB, sRGB
gamut mapping, and APCA-W3 contrast are ported from spec in `internal/color`.

## Usage

```sh
go run ./cmd/stellar init                       # write a default stellar.json
go run ./cmd/stellar build  -o stellar.css      # generate the CSS (readable)
go run ./cmd/stellar build  -o stellar.css --mode compact   # esbuild-minified
go run ./cmd/stellar export                     # print CSS to stdout
go run ./cmd/stellar minify app.ts -o app.min.js            # compile/minify TS/JS/CSS
go run ./cmd/stellar extract --image hero.png --harmony triad   # palette + seeds
go run ./cmd/stellar serve  --addr :7331        # editor + live-refresh
```

Open <http://localhost:7331> for the editor. The consuming page wires a single
SSE stream:

```html
<link id="stellar-css" rel="stylesheet">
<script src="/static/datastar.js" type="module"></script>
<body data-init="@get('/live-refresh')" data-signals-theme="'light'">
```

## Architecture (maps to stellar.md)

| Package              | Spec | Responsibility                                            |
|----------------------|------|-----------------------------------------------------------|
| `internal/scale`     | §4   | modular scale value + Utopia-style fluid `clamp()`        |
| `internal/color`     | §5   | OKLCH↔sRGB, gamut map, APCA-W3, ramps, on/dim, light/dark, image extraction + harmony |
| `internal/config`    | §2   | JSON schema, defaults, load/save, extraction resolution   |
| `internal/generate`  | §3,6 | one emitter per token family → sectioned partials         |
| `internal/minify`    | §9   | esbuild Go API — CSS minify + TS/JS compile               |
| `internal/server`    | §7   | HTTP + Datastar SSE live-refresh, content-hashed CSS, `/extract` |
| `internal/server/assets` | §8 | embedded editor SPA + vendored `datastar.js`            |
| `demo/`              | §11  | consumer smoke test (semantic, nested, `var(--token)`)    |

## Verification (stellar.md §11)

```sh
go test ./...
```

- **Scale parity** — generated tokens match the verified §4 numbers to 6 dp
  (`internal/scale`, `TestScaleParity`).
- **APCA** — black/white/grey reference pairs (`internal/color`).
- **Gamut** — every emitted `oklch()` maps back into sRGB `[0,1]`.
- **Contrast** — each `-on`/`-dim` meets its APCA target when physically
  reachable, else hits the maximum achievable contrast (§12: Lc 90 is
  unreachable against mid-tone greys).
- **Live-refresh** — verified headlessly: editing `size.minRatio` requests a new
  hashed stylesheet, patches `#stellar-css`, and changes computed `--size-6`
  with no navigation.
- **Consumer** — `demo/` renders against a generated `stellar.css`.

## esbuild + palette extraction

- **Minification (§9):** `--mode compact` and `serve` with compact output run the
  generated CSS through the embedded esbuild Go API (`internal/minify`). The
  `colorFallbacks` hex-before-`oklch()` pattern survives minification. `stellar
  minify <file>` compiles/minifies arbitrary `.ts/.tsx/.jsx/.js/.css` assets.
- **Palette extraction (§5.4):** k-means clustering in OKLab over a downscaled
  image, scored by population × chroma, with star ratings and hue names. Harmony
  modes (complement / triad / tetrad / analogous) rotate the primary hue to
  derive the other role seeds. Available three ways: `stellar extract`, the
  server `POST /extract` (multipart) endpoint, and an upload control in the
  editor that sets `colors.roles.*.seed` and live-restyles. Setting
  `colors.extract.image` in the config also resolves seeds at build time.

## Notes / deviations

- Ramp lightness uses a smoothstep curve between L 0.96 and 0.18 (§5.1); the
  original's image-tuned per-step overrides are supported by config but not
  byte-replicated (§12).

# Stellar

A local **design-token compiler** with a live visual editor. It expands a small
JSON config into a single `stellar.css` of `:root` custom properties — every
value derived parametrically — and hot-swaps the stylesheet over a Datastar SSE
stream on every edit. You then write semantic, natively-nested CSS that only
references `var(--token)`.

This is a self-contained Go implementation of the spec in
[`../datastar/stellar.md`](../datastar/stellar.md). No external dependencies:
OKLCH↔sRGB, sRGB gamut mapping, and APCA-W3 contrast are ported from spec in
`internal/color`.

## Usage

```sh
go run ./cmd/stellar init                       # write a default stellar.json
go run ./cmd/stellar build  -o stellar.css      # generate the CSS (readable)
go run ./cmd/stellar build  -o stellar.css --mode compact   # esbuild-minified
go run ./cmd/stellar export                     # print CSS to stdout
go run ./cmd/stellar minify app.ts -o app.min.js            # compile/minify TS/JS/CSS
go run ./cmd/stellar extract --image hero.png --harmony triad   # palette + seeds
go run ./cmd/stellar usage demo/                # report var(--token) usage (no purging)
go run ./cmd/stellar serve  --addr :7331        # editor + live-refresh
```

Open <http://localhost:7331> for the editor. The consuming page wires a single
SSE stream and an optional theme signal:

```html
<link id="stellar-css" rel="stylesheet">
<script src="/static/datastar.js" type="module"></script>
<body data-init="@get('/live-refresh')"
      data-signals:theme="'auto'"
      data-attr:data-theme="$theme === 'auto' ? false : $theme">
```

The generated CSS honours all three scheme states: no attribute follows
`prefers-color-scheme`, and `data-theme="light"` / `data-theme="dark"` (on any
ancestor, including `<body>`) force a scheme — so the theme signal actually
restyles, with `color-scheme` tracking along for native form controls.

`stellar serve` also mounts `./demo` at `/demo/` — the consumer smoke-test page
(semantic, no divs, HTML↔CSS nested 1:1, tokens only). A missing
`demo/stellar.css` falls back to the live generated CSS.

## The editor is pure Datastar (dogfood)

The editor SPA ships **zero hand-written JavaScript**. The config lives in
signals seeded server-side, every control is `data-bind`'ed, and one debounced
`data-on:input="@post('/config')"` on the form column is the only write.
Following the Tao of Datastar:

- **CQRS** — `POST /config` is a command and returns `204`; every read arrives
  over the long-lived `/live-refresh` SSE stream as element patches
  (stylesheet `<link>`, size meter).
- **Backend drives the frontend** — palette extraction posts the upload form
  (`contentType: 'form'`), and the server answers with an SSE patch rendering
  the candidate swatches; each swatch button writes harmony-derived seeds back
  into the signals and re-posts `/config`.
- **Previews are plain token consumers** — ramp swatches, chart hues, gradient
  swatches, type specimens, and radii reference `var(--token)` directly, so
  they restyle purely by the stylesheet swap. No preview scripting.
- **Reset defaults** — `POST /reset` restores `config.Default()` and answers
  with a `datastar-patch-signals` event (the Tao's "patch signals when
  appropriate" case) so every bound input snaps back, plus fresh link/meter
  patches.

## Architecture (maps to stellar.md)

| Package              | Spec | Responsibility                                            |
|----------------------|------|-----------------------------------------------------------|
| `internal/scale`     | §4   | modular scale value + Utopia-style fluid `clamp()`        |
| `internal/color`     | §5   | OKLCH↔sRGB, gamut map, APCA-W3, ramps, on/dim, light/dark, image extraction + harmony |
| `internal/config`    | §2   | JSON schema, defaults, load/save, extraction resolution   |
| `internal/generate`  | §3,6 | one emitter per token family → sectioned partials         |
| `internal/minify`    | §9   | esbuild Go API — CSS minify + TS/JS compile               |
| `internal/server`    | §7   | HTTP + Datastar SSE live-refresh, per-hash CSS store, `/extract`, `/demo` |
| `internal/server/assets` | §8 | embedded declarative editor + vendored `datastar.js`    |
| `demo/`              | §11  | consumer smoke test (semantic, no divs, nested 1:1, `var(--token)` only) |

## Token families

Beyond the spec's scales/colors/charts/code/named/aspect/z-index, §6's simple
families are emitted (each with its own section toggle):

- `--border-radius-{n}` (flat modular scale) + `--border-radius-round`
- `--border-size-{n}` from `border.sizes` (px)
- `--anim-duration-{n}` / `--anim-duration-base` + `--anim-ease-{name}` easings
- `--gradient-{role}` per theme role plus custom `--gradient-{name}` pairs —
  emitted as `var(--role-N)` **references**, so gradients restyle live with the
  ramps and inherit light/dark for free

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
- **Theme attribute** — light/dark are emitted three ways (`:root,
  [data-theme="light"]`, media-dark gated on `:not([data-theme="light"])`, and
  `[data-theme="dark"]`) so the toggle contract holds (`TestThemeAttrBlocks`).
- **Server** — hashed URLs are immutable and stay servable across edits (last 8
  builds), `current` is `no-store`, unknown hashes redirect, a bad config patch
  is atomic (400, nothing mutated), and `/live-refresh` opens with link+meter
  patches (`internal/server` tests).
- **Live-refresh** — verified in a real browser: editing `size.minRatio`
  requests a new hashed stylesheet, patches `#stellar-css`, and changes
  computed `--size-6` with no navigation and no page JS.
- **Consumer** — `demo/` renders against a generated `stellar.css`, standalone
  or served at `/demo/`.

## esbuild + palette extraction

- **Minification (§9):** `--mode compact` and `serve` with compact output run the
  generated CSS through the embedded esbuild Go API (`internal/minify`). The
  `colorFallbacks` hex-before-`oklch()` pattern survives minification. `stellar
  minify <file>` compiles/minifies arbitrary `.ts/.tsx/.jsx/.js/.css` assets.
- **Palette extraction (§5.4):** k-means clustering in OKLab over a downscaled
  image, scored by population × chroma, with star ratings and hue names. Harmony
  modes (complement / triad / tetrad / analogous) rotate the primary hue to
  derive the other role seeds. Available three ways: `stellar extract`, the
  server `POST /extract` (multipart) endpoint, and the editor's upload control,
  whose server-rendered candidates set `colors.roles.*.seed` and live-restyle.
  Setting `colors.extract.image` in the config also resolves seeds at build time.

## Notes / deviations

- Ramp lightness uses a smoothstep curve between L 0.96 and 0.18 (§5.1); the
  original's image-tuned per-step overrides are supported by config but not
  byte-replicated (§12).
- Dark tokens are emitted twice (under the media query and under
  `[data-theme="dark"]`) to make the explicit toggle work; compact mode + gzip
  absorb the duplication.

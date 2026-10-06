# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Vocal Practice App: a practice tool for singers. Admins upload multi-track MIDI files and define
song section/phrase structures; users pick a vocal track to be assessed against and backing
tracks to sing along with, record themselves, and get pitch/duration analysis scored against the
MIDI reference. Backend is Go + chi; frontend is vanilla JS (ES Modules) with no build step.

Full API reference and data-flow walkthrough (in Chinese) live in [README.md](README.md) — read it
for endpoint payloads/routes; this file focuses on things not obvious from a single file.

## Commands

```bash
# Run the production server (:8080) — requires JWT_SECRET
JWT_SECRET=devsecret go run cmd/server/main.go

# Run the test server (:18080 HTTP / :18443 HTTPS, seeded with test data + admin/user accounts)
go run cmd/test_server/main.go

# Go tests (whole repo)
go test ./...
# Single package / single test
go test ./internal/service/... -run TestAssessment -v

# Frontend unit tests (vitest, jsdom) — covers assets/js/__tests__ and ui-screens/js/__tests__
npm test
# Single test file
npx vitest run assets/js/__tests__/practice-business.test.js

# Other tools
go run cmd/midi_wav_compare/main.go   # generates output/comparison_report.html
go run cmd/snapshot/main.go           # generates snapshot/snapshot.html (project overview snapshot)
```

Default seeded admin (in-memory store): `admin@localhost` / `admin1234`. Override via
`--admin-email` / `--admin-password` flags on `cmd/server`.

There is no linter/formatter config beyond `gofmt`; there is no bundler for the frontend — HTML
pages load ES modules directly via `<script type="module">`.

### Known-broken test

`internal/service/ref_filter_test.go` shells out to `node` on
`cmd/midi_ref_test/ref_filter.cjs`, but that script now only exists at
`donottouchthis/midi_ref_test/ref_filter.cjs` (moved, not deleted). Those subtests fail with
`MODULE_NOT_FOUND` on a clean checkout; this is a pre-existing path drift, not something you broke.

## Architecture

### Backend layering (`internal/`)

`domain` (entities) → `storage` (persistence) → `service` (pure logic, no I/O) → `handler` (HTTP,
wires storage+service together). `cmd/server/main.go` is the composition root: it constructs one
`storage.Store` and one instance of each handler, then registers routes on a `chi.Router`.

- **`storage.Store`** (`internal/storage/store.go`) is the single interface all handlers depend on.
  Two implementations: `memory.go` (in-memory, default) and `postgres.go` (used when `DATABASE_URL`
  is set). Anything that needs new persisted state must be added to this interface and implemented
  in both.
- In-memory mode is not ephemeral: `internal/storage/backup` snapshots the `MemoryStore` to
  `./data/memory_backup.json` on an interval and on shutdown, and restores it on boot. Don't assume
  in-memory == "resets on restart."
- **`service`** holds two independent concerns that both feed into `handler/assessments.go`:
  1. `midi_parser.go` — parses uploaded `.mid` files into tracks/notes/tempo map at upload time
     (`admin_songs.go` calls this on `POST /api/v1/admin/songs`).
  2. `audio_decoder.go` + `pitch.go` + `vowel_consonant.go` + `assessment.go` — a **server-side**
     pitch-detection/scoring pipeline (autocorrelation pitch tracking, vowel/consonant segment
     separation, note alignment/scoring against MIDI). This backs
     `POST /api/v1/assessments/analyze` and `/analyze/basic`. The README's user-flow section
     describes client-side pitch detection (browser autocorrelation) feeding
     `POST /api/v1/assessments/submit` with pre-computed results — both paths exist; the frontend
     currently uses the client-side path, but the server can independently redo the analysis from
     an uploaded recording.
- `song_structure.go` domain models a two-level hierarchy: `SECTION` nodes contain `PHRASE`
  children (`StructureNode` is the tree shape returned by `Store.BuildStructureTree`). Deleting a
  SECTION cascades to its PHRASEs. Lyrics (`track_lyrics.go`) attach only to PHRASE + track pairs,
  never to SECTIONs.
- Songs support versioning: multiple `Song` rows can share a version-group id, with one marked
  active per group (`SetActiveVersion`); user-facing endpoints only ever see the active version.
- Auth is JWT-based (`handler/auth.go`), role-checked via `RequireRole("admin")` middleware wrapping
  the entire `/api/v1/admin` route group in `cmd/server/main.go`.

### `donottouchthis/`

Legacy/reference Go programs and scripts kept around but explicitly named to be left alone —
treat as read-only unless the user specifically asks you to work on something in there. Note that
some files elsewhere (e.g. `internal/service/ref_filter_test.go`) still reference old paths into
this directory that predate the move — see "Known-broken test" above.

### Frontend: two separate ES-module app surfaces, no shared bundler

1. **User practice flow** (`assets/js/`, served from `index.html` → `ui-screens/user-practice.html`).
   `state.js` (module-singleton store) is imported by everything else: `api.js` (fetch wrapper),
   `audio.js` (Web Audio synth playback of the backing track), `practice-business.js`/`practice-ui.js`
   (practice flow logic/rendering), and `selection.js`/`selection-ui.js` (section/phrase selection —
   see below). `midiParser.js` and `songDataExtractor.js` are legacy non-module scripts that attach
   to `window` (`window.MidiParser`, `window.SongDataExtractor`) rather than exporting — other ESM
   files read them off `window`, not via `import`.
2. **Admin backend UI** (`ui-screens/js/`, served from `admin-*.html`). Separate module graph, entry
   point `admin.js`, calling into `services/api-service.js`, `utils/error-handler.js`,
   `utils/songdata-converter.js`, `adapters/ui-renderer.js`, `adapters/audio-adapter.js`. This graph
   does **not** import from `assets/js/` (except `audio-adapter.js` depends on the global
   `window.MidiParser` from `assets/js/midiParser.js`).

These two surfaces duplicate some concepts (MIDI parsing, tick↔seconds conversion, WebAudio
playback) independently — don't assume a fix in one applies to the other.

**Section/phrase selection** (`assets/js/selection.js` + `selection-ui.js`) is a set-based model:
`state.selectedSectionIds` (whole sections) and `state.selectedPhraseIds` (individually toggled
phrases) are independent sets; `recomputeSelection()` derives `selectedStructure`/`selectedPhrasesData`
from their union, and selecting all phrases of a section auto-promotes it to "whole section"
selected (and vice versa on deselect). This replaced an earlier flat-index-range model — see
[docs/selection-logic.md](docs/selection-logic.md) for the full before/after rationale if you need
to touch selection behavior; the "proposed" design in that doc is what's now implemented.

### Client-side MIDI reference extraction

The frontend never fetches pre-computed reference note data from the server. It downloads the full
`.mid` file via `GET /api/v1/songs/{id}/midi` and parses it itself (`midiParser.js`), matching track
UUIDs from the API to MIDI track indices by assuming the same order the API returned them in. Keep
this ordering assumption in mind if track list ordering logic changes on the backend.

## Other docs worth knowing about

- [docs/timing-alignment.md](docs/timing-alignment.md), [docs/recording-comparison-flow.md](docs/recording-comparison-flow.md),
  [docs/implementation-phases.md](docs/implementation-phases.md), [docs/mobile-optimization-plan.md](docs/mobile-optimization-plan.md) —
  design/planning notes for specific subsystems; check the relevant one before making non-trivial
  changes to that area.
- `assets/js/FUNCTION_MAP.md` — function-level map of the `assets/js/` user-practice modules.
- `ui-screens/README.md` — admin UI-specific notes.

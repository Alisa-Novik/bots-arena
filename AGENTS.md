# Repository Guidelines

## Project Structure & Module Organization
- `cmd/golab` hosts the executable entry point; simulation packages live in `internal/` (`core`, `game`, `tasking`, `ui`, `config`, `util`, `assert`).
- Runtime resources load from `data/` and the legacy `assests/` folder; keep large experimental dumps out of git and document any new datasets in `README.md`.
- Profiling artifacts such as `cpu.out` and `debug_graphics.log` are disposable; regenerate them locally when diagnosing issues.

## Build, Test, and Development Commands
- `go run ./cmd/golab` launches the client; add `-h` to run headless when iterating on AI or scheduling logic without an OpenGL context.
- `go build -o bin/golab ./cmd/golab` produces a reusable binary (create `bin/` on first use); run `go mod tidy` after adding dependencies to keep the module file clean.
- Before every push, execute `go test ./...` and `go fmt ./... && go vet ./...`; call out any long-running headless simulations in your PR description.

## Coding Style & Naming Conventions
- All Go code must remain `gofmt`-clean (tabs for indentation, camelCase identifiers); avoid alternative formatters without consensus.
- Packages stay lowercase with short nouns (`tasking`, `ui`); exported symbols explain their role (`GameState`, `GenomeStore`), while helpers remain unexported.
- Use descriptive filenames that mirror their primary type (`colony_logic.go`, `board_renderer.go`) and keep configuration or texture files in snake_case to match loaders.

## Testing Guidelines
- Add `_test.go` files alongside source packages; prefer table-driven cases for instruction handlers and board utilities.
- Exercise multi-tick flows in headless mode with deterministic seeds so CI can run without GPU dependencies.
- Document any fixtures placed in `data/` and prune snapshots so they stay reviewable.

## Commit & Pull Request Guidelines
- Follow the concise imperative style seen in `git log` (e.g. `startup fix`); keep subjects under ~70 characters and expand context in the body when necessary.
- PRs should list the build/test commands you ran, link related issues or TODO items, and attach screenshots or brief clips for UI-facing updates.
- Flag profiling traces or config toggles reviewers need to reproduce your results, and tick off manual verification (build, run headless, run tests) before requesting review.

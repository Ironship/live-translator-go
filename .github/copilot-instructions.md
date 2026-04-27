# Copilot Instructions

These instructions are for AI coding agents working in this repository.

## Working Style

- Think before coding: state assumptions when the task is ambiguous and ask before making broad product or architecture choices.
- Keep changes surgical: touch only the files needed for the request, and do not refactor adjacent code unless it directly supports the fix.
- Prefer simple, explicit Go over speculative abstractions. This project follows a KISS scope: one Windows GUI, one Live Captions source, one local settings file, and a small provider set.
- Define a quick verification target before editing, then loop until it passes.

## Project Facts

- This is a Windows-only Go desktop app using `github.com/lxn/walk`; most source files use `//go:build windows`.
- Main entry point: `cmd/live-translator-go`.
- UI shell and preview live in `internal/overlay`; settings UI and app wiring live in `internal/app`.
- Capture logic lives in `internal/captions`; translation providers live in `internal/translator`; processing and chunking live in `internal/pipeline`.
- `setting.json` is local runtime configuration and may contain API keys. Treat it as private, do not commit secrets, and prefer source defaults/tests over editing a user's local settings.

## UI Guidance

- Match the native `walk` style already used in the app. Do not introduce a web UI framework.
- Keep the translator window quiet, compact, and readable: stable icon buttons, restrained spacing, clear hierarchy, and high-contrast caption text.
- Avoid adding long explanatory copy inside the app. Settings labels and short helper text are fine when they prevent misconfiguration.
- Preserve DPI-aware bounds handling and always-on-top/click-through behavior when changing the overlay window.

## Verification

- Run `gofmt` on changed Go files.
- Run `go test ./...` after Go changes.
- For release-sensitive changes, run `./scripts/build-release.ps1` on Windows.
- For visual changes, run the app with `go run ./cmd/live-translator-go` and capture screenshots with `./scripts/capture-screenshot.ps1` when practical.
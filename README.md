# Live Translator Go

Simple Windows 11 desktop app that reads text from Windows Live Captions, translates it, and shows the result in the same window.

The flow is intentionally minimal:

`Windows Live Captions -> translation -> on-screen preview`

## What It Is For

This project is for people who want fast translated captions from Windows Live Captions without building a separate ASR pipeline, backend service, or browser app.

Typical use cases:

- translating live speech during meetings, videos, and presentations,
- testing local providers such as Ollama or LM Studio,
- keeping captions and translated output in one desktop window,
- using a small local tool with minimal setup and no extra database.

## KISS Scope

This project is meant to stay close to KISS. The current version follows a few strict rules:

- one main GUI window,
- one transcription source: Windows Live Captions,
- one local settings file: `setting.json`,
- only four providers: Google, DeepL, Ollama, and LM Studio,
- no history database, installer, or background service.

## What The Program Does

- reads text from the system Live Captions window through Windows UI Automation,
- sends text to the selected translation provider,
- shows the result in a dark preview window,
- lets you change provider and runtime settings inside the `Settings` panel,
- saves local settings to `setting.json`.

## What It Does Not Do

- it does not implement its own speech-to-text engine,
- it does not require a backend server,
- it does not keep translation history in a database,
- it does not try to be a full subtitle suite.

## Requirements

- Windows 11 with Windows Live Captions available,
- Go 1.22+ if you want to run from source,
- provider configuration only if you are not using Google.

## Quick Start

Run from the project directory:

```powershell
go run ./cmd/live-translator-go
```

If Live Captions is not enabled yet, start it in Windows and wait for the app to attach to the window.

Google works out of the box. For other providers, configure the values in `Settings`.

Example for Ollama:

```powershell
$env:LIVE_TRANSLATOR_PROVIDER = "Ollama"
$env:LIVE_TRANSLATOR_BASE_URL = "http://localhost:11434/v1"
$env:LIVE_TRANSLATOR_MODEL = "llama3.1:8b"
go run ./cmd/live-translator-go
```

## Build

Desktop build without a visible console window:

```powershell
go build -ldflags="-H windowsgui" ./cmd/live-translator-go
```

## Project Structure

- `cmd/live-translator-go` - application entry point,
- `internal/captions` - Live Captions text capture,
- `internal/translator` - translation providers,
- `internal/pipeline` - input coalescing and processing,
- `internal/overlay` - main window and preview,
- `internal/app` - app wiring and settings flow,
- `setting.json` - local runtime settings, intentionally not committed.

## License

This project is released under GPL-3. See `LICENSE`.
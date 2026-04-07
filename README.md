# Live Translator Go

Prosty desktopowy program dla Windows 11, ktory czyta tekst z Windows Live Captions, tlumaczy go i pokazuje wynik od razu w tym samym oknie.

Przeplyw jest celowo prosty:

`Windows Live Captions -> tlumaczenie -> podglad na ekranie`

## Do czego to sluzy

Program jest dla osob, ktore chca szybko zobaczyc tlumaczenie napisow generowanych przez Windows Live Captions bez budowania osobnego pipeline ASR, serwera albo przegladarkowej aplikacji.

Najwazniejsze zastosowania:

- tlumaczenie mowy na zywo podczas spotkan, filmow i prezentacji,
- testowanie lokalnych providerow takich jak Ollama albo LM Studio,
- prosty podglad napisow i tlumaczenia w jednym oknie,
- minimalna konfiguracja bez dodatkowej bazy danych i bez drugiego procesu UI.

## Zasada KISS

Ten projekt ma byc zgodny z KISS. Obecna wersja trzyma sie kilku prostych zasad:

- jedno glowne okno GUI,
- jedno zrodlo transkrypcji: Windows Live Captions,
- jeden lokalny plik ustawien `setting.json`,
- tylko czterech providerow: Google, DeepL, Ollama, LM Studio,
- bez bazy historii, bez instalatora, bez uslug w tle.

## Co program robi

- odczytuje tekst z systemowego okna Live Captions przez Windows UI Automation,
- wysyla tekst do wybranego providera tlumaczen,
- pokazuje wynik w ciemnym oknie z podgladem,
- pozwala zmienic providera i parametry w panelu `Settings`,
- zapisuje lokalne ustawienia do `setting.json`.

## Czego program nie robi

- nie implementuje wlasnego speech-to-text,
- nie wymaga serwera backendowego,
- nie prowadzi historii w bazie danych,
- nie probuje byc pelnym kombajnem do napisow.

## Wymagania

- Windows 11 z dostepnym Windows Live Captions,
- Go 1.22+ do uruchamiania ze zrodel,
- skonfigurowany provider tylko wtedy, gdy nie uzywasz Google.

## Szybki start

Uruchom z katalogu projektu:

```powershell
go run ./cmd/live-translator-go
```

Jesli Live Captions nie jest wlaczone, uruchom je z poziomu Windows i poczekaj, az aplikacja podepnie sie do okna.

Google dziala od razu. Dla innych providerow skonfiguruj dane w `Settings`.

Przyklad dla Ollama:

```powershell
$env:LIVE_TRANSLATOR_PROVIDER = "Ollama"
$env:LIVE_TRANSLATOR_BASE_URL = "http://localhost:11434/v1"
$env:LIVE_TRANSLATOR_MODEL = "llama3.1:8b"
go run ./cmd/live-translator-go
```

## Build

Desktopowy build bez widocznego okna konsoli:

```powershell
go build -ldflags="-H windowsgui" ./cmd/live-translator-go
```

## Struktura projektu

- `cmd/live-translator-go` - punkt startowy aplikacji,
- `internal/captions` - odczyt tekstu z Live Captions,
- `internal/translator` - providerzy tlumaczen,
- `internal/pipeline` - laczenie i przetwarzanie wejscia,
- `internal/overlay` - glowne okno i podglad,
- `internal/app` - skladanie konfiguracji i logiki aplikacji,
- `setting.json` - lokalne ustawienia uruchomieniowe, nieprzeznaczone do commita.

## Licencja

Projekt jest udostepniony na licencji GPL-3. Zobacz plik `LICENSE`.
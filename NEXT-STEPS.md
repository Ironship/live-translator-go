# Next Session Plan

Stan na 2026-04-07:

- watcher Live Captions dziala natywnie w Go przez Windows UI Automation
- aplikacja dziala jako jedno okno GUI bez widocznego PowerShell
- providerzy: Google, DeepL, Ollama, LM Studio
- ustawienia sa wbudowane w GUI i zapisywane do setting.json
- w zakladce Translation jest juz bazowy przycisk Test Connection dla providera

## Priorytet 1 - Dopieszczenie GUI

Cel: zrobic interfejs, ktory wyglada jak normalna aplikacja desktopowa, a nie tylko poprawny technicznie layout.

Zakres:

- uporzadkowac typografie, odstepy i wysokosci kontrolek
- dodac wyrazniejsza hierarchie wizualna miedzy headerem, preview i settings
- poprawic kolory tla, kontrast i spojnosc przyciskow
- dopracowac preview tak, zeby bardziej przypominal finalny pasek napisow

## Priorytet 2 - Tryb compact i expanded

Cel: rozdzielic dwa scenariusze pracy.

Zakres:

- compact mode: maly pasek z tlumaczeniem, zawsze na wierzchu
- expanded mode: normalne okno sterowania z ustawieniami i podgladem
- szybkie przelaczanie miedzy trybami z poziomu GUI
- zapamietywanie ostatniego trybu po restarcie

## Priorytet 3 - Rozszerzenie testu polaczenia providera

Cel: skrocic czas debugowania konfiguracji.

Zakres:

- dopracowac obecny przycisk Test Connection w zakladce Translation
- walidacja wymaganych pol zaleznosci od providera
- czytelny komunikat success / error bez zgadywania co poszlo nie tak
- dla Google test prostego zapytania, dla Ollama / LM Studio test endpointu i modelu, dla DeepL test klucza API

## Priorytet 4 - Lepszy UX ustawien

Cel: uproscic konfiguracje dla normalnego uzytkownika.

Zakres:

- dropdowny lub gotowe presety dla popularnych jezykow
- sensowne placeholdery i opisy pol
- zapamietywanie ostatniej aktywnej zakladki settings
- osobne komunikaty dla watcher status, provider status i validation errors

## Priorytet 5 - Stabilnosc runtime

Cel: poprawic zachowanie programu przy realnym uzyciu.

Zakres:

- automatyczne ponowne podpiecie do Live Captions po restarcie okna systemowego
- lepsze komunikaty w GUI, gdy zrodlo captions chwilowo znika
- sprawdzenie zachowania przy szybkim naplywie caption events
- testy na roznych skalach DPI i przy wielu monitorach

## Priorytet 6 - Investigacja automatycznego uruchamiania Live Captions

Cel: sprawdzic, czy da sie uruchamiac Windows Live Captions z poziomu aplikacji, bez recznego wlaczania przez uzytkownika.

Status po investigacji 2026-04-07:

- Windows rejestruje `Live captions` jako Start App z shell targetem `shell:AppsFolder\{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\LiveCaptions.exe`
- nie znaleziono udokumentowanego publicznego URI `ms-settings:` ani Win32 API dedykowanego bezposrednio do startu Live Captions
- bezpieczna implementacja w aplikacji to best-effort direct launch przez shell target i fallback do `ms-settings:accessibility-audio`
- watcher moze juz czekac na pojawienie sie okna i podpiac sie automatycznie po uruchomieniu Live Captions

Zakres investigacji:

- sprawdzic, czy istnieje oficjalny command, URI, shell target albo Windows API do uruchomienia Live Captions
- sprawdzic, czy da sie to wywolac przez ustawienia dostepnosci Windows lub inny stabilny entry point systemowy
- ustalic, czy uruchomienie moze byc bezpieczne i przewidywalne na Windows 11 bez hakow typu symulacja klawiszy
- jesli nie ma stabilnej oficjalnej drogi, zaplanowac sensowny fallback, np. przycisk Open Live Captions albo komunikat z 1-click guidance
- sprawdzic, czy po uruchomieniu aplikacja moze automatycznie poczekac na pojawienie sie okna Live Captions i podpiac watcher bez dodatkowych krokow

## Rekomendowana kolejnosc na nastepna sesje

1. Dopieszczenie GUI
2. Tryb compact i expanded
3. Test polaczenia providera
4. Investigacja automatycznego uruchamiania Live Captions

## Po tych zmianach

Jesli powyzsze bedzie gotowe, warto rozwazyc:

- eksport i log historii tlumaczen
- profile providerow
- pakowanie do prostego instalatora lub release ZIP z ikonami i zasobami
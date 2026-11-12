# Courses Hub

Self-hosted portal do moich interaktywnych kursów HTML. Kursy wrzucam przez
przycisk **„Dodaj kurs”** w portalu, a portal pokazuje je jako karty ze statusem
(nie rozpoczęty / w trakcie / ukończony).

Single-user, bez auth. Stack: **Go** (`net/http`, bez frameworka) + **SQLite**
(`modernc.org/sqlite`, pure Go, bez CGO) + czysty **HTML/CSS/JS**.

## Deployment (Docker Compose)

Na maszynie docelowej (potrzebny Docker z Compose):

```bash
git clone <adres-repo> courses-hub
cd courses-hub
docker compose up -d --build
```

Portal: `http://<adres-maszyny>:8080`. Gotowe — kursy dodajesz z poziomu przeglądarki.

**Aktualizacja** po zmianach w kodzie:

```bash
git pull
docker compose up -d --build
```

**Dane** (baza SQLite + wrzucone kursy) są w wolumenie `courses-data`, więc
przetrwają przebudowę i aktualizację obrazu. Kopia zapasowa:

```bash
docker run --rm -v courses-data:/data -v "$PWD":/backup alpine tar czf /backup/courses-backup.tgz -C /data .
```

> ⚠️ Aplikacja nie ma logowania — każdy, kto ma dostęp do portu 8080, może
> dodawać i usuwać kursy. Trzymaj ją w sieci domowej / za VPN (np. Tailscale)
> albo za reverse proxy z uwierzytelnianiem. Nie wystawiaj jej publicznie.

## Dodawanie kursów

1. Kliknij **„+ Dodaj kurs”**.
2. Wybierz plik HTML kursu (single-file). Tytuł wypełni się sam z `<title>` pliku.
3. Uzupełnij/zmień: **tytuł**, **slug** (adres kursu, generuje się z tytułu),
   **opis**, **kategorię** (np. `programming`, `ml`, `data` albo własną).
4. Kurs pojawi się jako karta i będzie dostępny pod `/courses/<slug>/`.

Na karcie są też przyciski **✓ Ukończony / ↺ Cofnij** (ręczna zmiana statusu)
i **Usuń** (kasuje plik kursu i zapisany postęp).

### Tracking

Nic nie trzeba wpinać ręcznie — serwer przy wyświetlaniu kursu sam dokleja
przed `</body>` skrypt [`/tracking.js`](web/portal/tracking.js) (plik kursu na
dysku zostaje nietknięty):

- wejście do kursu → status **w trakcie** (ukończony kurs nie jest cofany),
- `window.markCourseCompleted()` → **ukończony**. Możesz to wywołać w kursie
  (np. po ostatnim kroku albo zdanym quizie) — albo po prostu kliknąć
  „✓ Ukończony” na karcie w portalu.

## Uruchomienie lokalne (bez Dockera)

```bash
go run ./cmd/server
# → http://localhost:8080
```

Konfiguracja przez zmienne środowiskowe:

| Zmienna         | Domyślnie            | Opis                                  |
|-----------------|----------------------|---------------------------------------|
| `ADDR`          | `:8080`              | adres nasłuchu                        |
| `WEB_DIR`       | `./web`              | katalog z portalem (`web/portal`)     |
| `DB_PATH`       | `./data/courses.db`  | plik bazy SQLite                      |
| `COURSES_DIR`   | `./data/courses`     | katalog z wrzuconymi kursami          |
| `MAX_UPLOAD_MB` | `50`                 | limit rozmiaru pliku kursu            |

Baza jest tworzona i migrowana automatycznie przy starcie.

## Struktura

```
courses-hub/
  cmd/server/main.go        # konfiguracja, routing, graceful shutdown
  internal/db/              # połączenie, migracje, zapytania
  internal/handlers/        # API + serwowanie kursów z trackingiem
  web/portal/               # portal: index.html, style.css, app.js, tracking.js
  Dockerfile
  docker-compose.yml
  .github/workflows/deploy.yml
```

## API

| Metoda   | Ścieżka                 | Opis                                                    |
|----------|-------------------------|---------------------------------------------------------|
| `GET`    | `/api/courses`          | lista kursów ze statusem                                |
| `POST`   | `/api/courses`          | dodanie kursu (multipart: `file`, `title`, `slug`, `description`, `category`) |
| `DELETE` | `/api/courses/{slug}`   | usunięcie kursu (plik + postęp)                         |
| `POST`   | `/api/progress`         | `{"slug": "...", "status": "not_started\|in_progress\|completed"}` |
| `GET`    | `/courses/{slug}/`      | strona kursu z doklejonym trackingiem                   |
| `GET`    | `/healthz`              | `ok`                                                    |

Przykład dodania kursu z terminala:

```bash
curl -F file=@kurs.html -F title="Mój kurs" -F slug=moj-kurs -F category=programming \
  http://localhost:8080/api/courses
```

## CI/CD (opcjonalnie)

[`.github/workflows/deploy.yml`](.github/workflows/deploy.yml) na każdy push do
`main` buduje obraz `linux/amd64` i pushuje do **GHCR** z tagami `latest` oraz
`sha-<krótki-hash>`. Do deploymentu przez `docker compose up -d --build` nie
jest potrzebny. Obraz z GHCR można uruchomić wprost:

```bash
docker run -d -p 8080:8080 -v courses-data:/data ghcr.io/bartohehe/courseshub:latest
```

Pakiety w GHCR są domyślnie prywatne — przed `docker pull` trzeba się zalogować
(`docker login ghcr.io`, token z uprawnieniem `read:packages`) albo ustawić pakiet
jako publiczny w ustawieniach pakietu na GitHubie.

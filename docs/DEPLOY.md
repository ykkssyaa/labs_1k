# Развёртывание «с нуля»

Проверено на чистой машине с Linux; на Windows — через Docker Desktop.

## Вариант 1. Docker (рекомендуется)

Нужны Git и Docker с плагином Compose.

```bash
git clone <адрес репозитория> vestnik-cms
cd vestnik-cms
cp .env.example .env            # задайте VESTNIK_ADMIN_PASSWORD и VESTNIK_BASE_URL
docker compose up --build -d
docker compose logs -f vestnik  # дождаться строки "listening"
```

Открыть `http://localhost:8080` (сайт) и `http://localhost:8080/admin/` (панель).

При первом старте:
1. применяются миграции БД (`db_migrations`);
2. синхронизируются схемы типов (`content_types`);
3. при `VESTNIK_SEED=1` и пустой БД создаются демо-данные и администратор из `.env`.

Данные (БД и загруженные файлы) лежат в томе `vestnik-data` (`/app/data`).

Остановить: `docker compose down`. Удалить вместе с данными: `docker compose down -v`.

## Вариант 2. Без Docker

Нужен Go 1.25+.

```bash
git clone <адрес репозитория> vestnik-cms && cd vestnik-cms
cp .env.example .env
make build                      # bin/vestnik, статический бинарник без cgo
./bin/vestnik migrate
./bin/vestnik seed              # демо-данные (необязательно)
./bin/vestnik adduser me@example.org "Имя" admin 'длинный-пароль'
./bin/vestnik serve
```

Бинарник ищет `content_types/`, `config/`, `themes/` в текущем каталоге
(пути меняются переменными `VESTNIK_CONTENT_TYPES`, `VESTNIK_ROLES`, `VESTNIK_THEMES`).

## Настройки

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `VESTNIK_ADDR` | `:8080` | адрес сервера |
| `VESTNIK_BASE_URL` | `http://localhost:8080` | внешний адрес: canonical, sitemap, API; `https://` включает флаг Secure у cookie |
| `VESTNIK_DB` | `data/vestnik.db` | файл SQLite |
| `VESTNIK_UPLOADS` | `data/uploads` | каталог медиа |
| `VESTNIK_THEME` | `classic` | тема по умолчанию (дальше — из настроек в панели) |
| `VESTNIK_ADMIN_EMAIL`, `VESTNIK_ADMIN_PASSWORD` | — | администратор, создаваемый `seed` |
| `VESTNIK_SEED` | `0` (`1` в compose) | заполнить пустую БД демо-данными при старте |

## Эксплуатация

- **Резервная копия:** `sqlite3 data/vestnik.db ".backup backup.db"` + архив `data/uploads`.
- **Обновление:** `git pull && docker compose up --build -d` — новые миграции применятся сами.
- **Статистика:** сворачивается фоном раз в час; вручную — `vestnik rollup`.
- **HTTPS:** поставить перед сервисом обратный прокси (Caddy, nginx) и задать `VESTNIK_BASE_URL=https://...`.

## Проверка после развёртывания

```bash
curl -s localhost:8080/healthz            # ok
curl -s localhost:8080/robots.txt
curl -s 'localhost:8080/api/v1/entries?per_page=1'
```

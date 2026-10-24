# 02. Архитектура

## Слои

```
cmd/vestnik            точка входа: serve, migrate, seed, adduser, rollup
internal/app           сборка зависимостей, HTTP-сервер, middleware
internal/web/admin     панель администрирования (HTML-формы)
internal/web/site      публичный сайт, SEO (sitemap, robots, метатеги)
internal/web/api       REST API (JSON, headless)
internal/content       ядро: реестр схем, валидация, жизненный цикл, ревизии, таксономия
internal/access        роли и матрица прав (из config/roles.json)
internal/media         загрузка и производные размеры изображений
internal/analytics     сбор просмотров и агрегаты
internal/render        темы, наследование и partial-шаблоны, markdown, экранирование
internal/store         SQLite, миграции, репозитории
internal/config        конфигурация из окружения и .env
```

Правило зависимостей: `web/* → content, access, media, analytics, render → store`.
Ядро (`content`) не импортирует `web` и `render`. `store` не знает про HTTP.

## Хранилище

SQLite, один файл `data/vestnik.db`, режим WAL, внешние ключи включены.
Миграции схемы БД — SQL-файлы `internal/store/migrations/NNNN_name.sql`,
встраиваются в бинарник (`embed`) и применяются при старте автоматически;
применённые записываются в `db_migrations`. Изменять применённую миграцию
нельзя — только добавлять новую (это защищает хук харнесса).

Поля типов хранятся в колонке `entries.data` (JSON). Почему не таблица на тип и
не EAV — ADR-0002.

## Границы модулей

| Модуль | Публичный интерфейс | Чего не делает |
|---|---|---|
| `content` | `Registry`, `Service` (Create, Update, Transition, Rollback, Delete, List) | не рендерит, не знает про HTTP |
| `access` | `Policy.Can(user, action, Resource)` | не ходит в БД |
| `render` | `Engine.Render(w, theme, page, data)` | не читает БД |
| `analytics` | `Tracker.Track`, `Stats.*` | не меняет материалы |
| `store` | репозитории `*Repo` | не проверяет права и схемы |

## Конфигурация

Переменные окружения (или `.env`, секреты только там):

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `VESTNIK_ADDR` | `:8080` | адрес сервера |
| `VESTNIK_DB` | `data/vestnik.db` | файл БД |
| `VESTNIK_UPLOADS` | `data/uploads` | каталог медиа |
| `VESTNIK_THEME` | `classic` | тема по умолчанию |
| `VESTNIK_BASE_URL` | `http://localhost:8080` | для sitemap и canonical |
| `VESTNIK_ADMIN_EMAIL`, `VESTNIK_ADMIN_PASSWORD` | — | первый администратор при `seed` |
| `VESTNIK_SEED` | `0` | `1` — заполнить демо-данными при пустой БД |

## Точки контроля этапа 1

- `make check` = `gofmt` + `go vet` + `staticcheck` + `go test ./...`.
- `make run` поднимает стенд одной командой (миграции применяются сами).
- `docker compose up --build` — то же в контейнере.

# Вестник CMS

Собственная система управления контентом городского онлайн-издания. Код сгенерирован
ИИ-агентом (Claude Code) по спецификациям из `spec/`; решения — в `docs/adr/`, история
работы с агентом — в `docs/ai-journal.md` и `traces/`.

## Возможности

- Типы материалов задаются JSON-схемами в `content_types/` — новый тип без правки кода.
- Жизненный цикл: черновик → на модерации → опубликован → архив; ревизии с откатом.
- Рубрики (иерархия) и теги; медиатека с производными размерами изображений.
- Панель администрирования: роли из `config/roles.json`, модерация, пакетные операции,
  фильтры и поиск, журнал действий, статистика.
- Публичный сайт в двух темах (`classic`, `newspaper` наследует `classic`), меню из
  контент-модели, ЧПУ, метатеги, `sitemap.xml`, `robots.txt`.
- REST API (`/api/v1/...`) для headless-режима.
- Встроенная аналитика: просмотры без персональных данных, дневные свёртки, экран статистики.

## Быстрый старт

```bash
docker compose up --build        # http://localhost:8080, демо-данные создаются автоматически
# или без Docker (Go 1.25+):
make run
```

Демо-учётки (пароль `demo-password`, у администратора — `VESTNIK_ADMIN_PASSWORD`, если задан):

| Роль | Email |
|---|---|
| Администратор | `admin@vestnik.local` |
| Редактор | `editor@vestnik.local` |
| Автор | `author@vestnik.local` |

## Разработка

```bash
make check   # gofmt + go vet + staticcheck + все тесты
make test    # только тесты
```

| Каталог | Что внутри |
|---|---|
| `cmd/vestnik` | точка входа: `serve`, `migrate`, `seed`, `rollup`, `adduser` |
| `internal/content` | ядро: схемы, валидация, жизненный цикл, ревизии, таксономия |
| `internal/access` | матрица прав |
| `internal/web/{admin,site,api}` | панель, сайт, REST API |
| `internal/render` | темы и markdown |
| `internal/analytics` | просмотры и агрегаты |
| `internal/store` | SQLite, миграции, репозитории |
| `themes/` | темы `_admin`, `classic`, `newspaper` |
| `spec/`, `docs/adr/` | спецификации и архитектурные решения |
| `e2e/` | сквозные тесты |

Развёртывание с нуля — `docs/DEPLOY.md`. Изменения — `CHANGELOG.md`.

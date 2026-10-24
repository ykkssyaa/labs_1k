# 03. Этап 2 — ядро управления контентом

Контент-модель — `01-content-model.md`. Здесь — порции, на которые делится
генерация, и критерии приёмки каждой. Агент получает одну порцию за раз.

## Порция 2.1. Реестр схем и валидация (`internal/content/schema.go`, `validate.go`)

- `LoadRegistry(dir)` читает все `*.json`, проверяет схему (ошибки из раздела
  «Формат схемы»), возвращает `*Registry` с методами `Get(name)`, `List()`
  (сортировка по `label`), `ByPrefix(urlPrefix)`.
- `Type.Validate(data map[string]any) (clean map[string]any, errs FieldErrors)`:
  приводит значения из формы (строки) и из JSON (числа, bool) к типу поля,
  отбрасывает неизвестные поля, применяет `default`, проверяет правила.
- `FieldErrors` — `map[string]string` «поле → сообщение по-русски».
- Критерии: каждый тип поля и каждое правило покрыты табличным тестом;
  схема с ошибкой не загружается.

## Порция 2.2. Хранилище материалов и ревизий (`store/entries.go`, `content/service.go`)

- `store.EntryRepo`: Insert, Update, Get, GetBySlug(type, slug), Delete, List(filter).
  Фильтр: тип, статус, автор, рубрика, тег, строка поиска (по заголовку и
  `data`), сортировка, лимит/смещение; возвращает и общее число.
- `store.RevisionRepo`: Add (номер = max+1 в той же транзакции), List(entryID), Get(entryID, n).
- `content.Service`: `Create(ctx, actor, input)`, `Update`, `Rollback(ctx, actor, id, n)`,
  `Delete`. Каждое сохранение — одна транзакция «материал + ревизия + связи с терминами».
- Slug: если пуст — транслитерация заголовка; при конфликте добавляется `-2`, `-3`…
- Критерии: инварианты 1–3, 6–8 из контент-модели.

## Порция 2.3. Жизненный цикл (`content/lifecycle.go`)

- Таблица переходов `transitions` (имя → из → в). `Service.Transition(ctx, actor, id, name)`.
- Права на переход — через интерфейс `Authorizer` (реализует `access.Policy`
  на этапе 3; до этого — заглушка «всё можно» только в тестах).
- `published_at` — при первом `publish`.
- Критерии: инварианты 4–5; все недопустимые пары «статус → переход» дают `ErrBadTransition`.

## Порция 2.4. Миграции схем (`content/schema_migrate.go`)

- `Registry.Sync(ctx, db)` — сравнивает версии с таблицей `content_types`, применяет
  шаги `add_field`/`rename_field`/`remove_field` к `data` всех материалов типа,
  создаёт ревизию с пометкой «миграция схемы vN», пишет `schema_migrations`.
- Понижение версии — ошибка.
- Критерии: тест «версия 1 → 2 с add_field и rename_field» и тест на понижение.

## Порция 2.5. Таксономия (`store/terms.go`, `content/taxonomy.go`, отдельная миграция)

- Таблицы `terms`, `entry_terms` (FK, `ON DELETE CASCADE`, уникальность пары).
- `Taxonomy`: CreateTerm, UpdateTerm (запрет цикла по `parent_id`), DeleteTerm,
  ListTerms(vocabulary), TermBySlug, EnsureTags(names) — создаёт недостающие теги.
- Связи пишутся в той же транзакции, что и материал; попадают в снимок ревизии.
- Критерии: цикл рубрик отклоняется; удаление термина снимает связи; откат
  восстанавливает и термины.

## Порция 2.6. Медиа (`internal/media`, отдельная миграция)

- Таблицы `media`, `media_variants`.
- `media.Service.Upload(ctx, actor, filename, reader)`: проверка типа по
  содержимому (`http.DetectContentType`), лимит 10 МБ, декодирование, запись
  оригинала в `uploads/ГГГГ/ММ/<случайное имя>.<ext>`, производные `thumb`, `medium`,
  `large` по правилам контент-модели (без увеличения).
- `Delete` удаляет файлы и записи; `Get`, `List`.
- Критерии: размеры производных проверены тестом; не-изображение отклоняется.

## Порция 2.7. Тесты ядра

Не меньше 25 тестов (функций или подтестов табличных тестов) в `internal/content`
и `internal/media`: валидация полей, инварианты, round-trip сериализации
(`Entry` → JSON → `Entry`, снимок ревизии → материал), откат, переходы, миграции схем.

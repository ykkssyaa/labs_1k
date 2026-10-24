# Интересные места — веб-версия практикума SURF

Задания [flutter-lab-practicum](https://github.com/surfstudio/flutter-lab-practicum) (8 лабораторных), сделанные как веб-приложение:

- **бэкенд** — Go (стандартная библиотека + SQLite для отзывов), `backend/`;
- **фронтенд** — чистый JavaScript (ES-модули, без сборки), `web/`;
- всё запускается в Docker; в браузере работает одинаково на Windows и Linux.

## Запуск

```bash
docker compose up -d --build
```

Открыть http://localhost:8888. Остановить: `docker compose down`.

Переменные окружения (можно задать перед командой или в `.env`):

| Переменная | По умолчанию | Что делает |
|---|---|---|
| `DELAY_MS` | 300 | задержка ответов API, чтобы были видны загрузчики и скелетоны |
| `FAIL_RATE` | 0 | доля ответов 503, например `FAIL_RATE=0.3` — проверить ошибки и повторы запросов |

## Тесты

```bash
# бэкенд (go test) запускается при сборке образа
docker compose up -d --build

# unit + widget + golden + интеграционные тесты фронтенда
docker compose --profile test run --rm tests

# обновить эталонные скриншоты golden-тестов
PW_ARGS=--update-snapshots docker compose --profile test run --rm tests
```

Без Docker: `cd backend && go test ./...`, `npm run test:unit` (нужен Node 20+).

## Что где сделано

Нумерация как в таблице курса: ПР1–ПР4 = labs/lab1–lab4, ЛР1 = lab5 (хранилище), ЛР2 = lab6 (тесты).

| Работа | Задание | Где |
|---|---|---|
| **ПР1** Вёрстка виджетов | нижняя панель навигации | `web/js/feature/bottom_nav/bottom_nav.js` |
| | поле поиска, ввод выводится в консоль | `web/js/feature/search/search_screen.js` (`console.log`) |
| | карточка места | `placeCard()` в `web/js/uikit/widgets.js` |
| **ПР2** Экраны и состояние | онбординг: 3 страницы, точки, «Пропустить», «На старт» | `web/js/feature/onboarding/` |
| | экран места: галерея, детали, «В избранное» | `web/js/feature/place_detail/` |
| | фильтр: радиус, категории, сброс, «Показать (n)» внизу | `web/js/feature/filter/filter_screen.js` |
| **ПР3** Навигация | вкладки, переходы к деталям/поиску/фильтру, «Назад» | `web/js/navigation/router.js`, `web/js/main.js` |
| | маршруты для всех экранов, передача параметров | таблица `routes` в `main.js`, `router.go(path, extra)` |
| | сплэш и первый запуск, отдельный стек у каждой вкладки, анимации переходов | `feature/splash/`, `tabStacks` в `router.js`, View Transitions в `css/app.css` |
| **ПР4** Работа с сетью | HTTP-клиент с таймаутом 30 с, `getPlaces`, `getPlaceDetails`, загрузчики | `web/js/api/api_client.js` |
| | обновление списка потягиванием вниз, `FilterPlacesRequestDto`, POST `/filtered_places`, фильтрация на сервере | `places_screen.js`, `FilterPlacesConverter` в `domain/place.js`, `backend/main.go` |
| | классы ошибок, `unwrapError`, `ErrorPlaceholder`, повтор с экспоненциальной задержкой | `core/failures.js`, `errorPlaceholder()`, `apiRetryInterceptor()` |
| **ЛР1** Локальное хранилище | избранное, тема, история поиска сохраняются | IndexedDB (аналог Drift): `persistence/*_dao.js`; localStorage (аналог SharedPreferences): `key_value_storage.js` |
| | `FilterStorage` в localStorage, применение при запуске, «Очистить все фильтры» в настройках | `LocalFilterStorage` в `persistence/filter_storage.js`, `settings_screen.js` |
| | таблица фильтров с индексами и конвертерами, DAO (CRUD, активный фильтр, пресеты, поиск по имени и дате), кэш, миграция v1→v2 с переносом из localStorage и откатом | `persistence/filter_settings_dao.js`, `DbFilterStorage`, `persistence/database.js`, `initDatabase()` в `main.js` |
| **ЛР2** Тестирование | unit: конвертеры избранного, шаги онбординга, `SearchRepository`, `SettingsModel` | `test/unit/` |
| | widget: кнопка избранного, нижняя навигация; golden: карточка места | `test/e2e/widget.spec.js`, `test/e2e/golden.spec.js` |
| | интеграционный: список → детали → избранное | `test/e2e/integration.spec.js` |
| lab7 Анимации | Hero-переход карточка → экран места | `view-transition-name: place-<id>` |
| | цвет карточки и «пульс» иконки при добавлении в избранное, shimmer у скелетона | `.place-card--favorite`, `.pulse`, `.skeleton` в `app.css` |
| | анимированные переходы к деталям и к поиску, ступенчатое появление в онбординге | `html[data-nav=…]`, `.stagger` в `app.css` |
| lab8 Локализация | все строки в ARB-файлах, ru и en, вложенность по фичам, описания у всех ключей | `web/js/l10n/ru.json`, `en.json` |
| | язык системы (иначе русский), 5 примеров множественного числа, числа, валюта, даты | `l10n.js`, экран настроек «О приложении» |
| | переключение языка в настройках, вариант «Как в системе», сохраняется между запусками | `settings_screen.js`, `LocaleStorage` |

## Дополнительно

- **Карта мест** — вкладка «Карта» (Leaflet + OpenStreetMap), все места маркерами, из всплывающей карточки можно перейти к месту.
- **Отзывы и рейтинг** — оценка 1–5 и комментарий на экране места, средний рейтинг на карточках, сортировка по рейтингу в фильтре. Отзывы хранятся в SQLite на сервере (Docker-том `reviews`).

## API

| Метод | Путь | Описание |
|---|---|---|
| GET | `/place` | все места |
| GET | `/place/{id}` | одно место |
| POST | `/filtered_places` | `{lat, lng, radius, typeFilter, nameFilter, sortBy}` → места |
| GET | `/place/{id}/reviews` | отзывы о месте |
| POST | `/place/{id}/reviews` | `{author, rating, text}` → новый отзыв |

Ошибки возвращаются как `{"error": "...", "reasons": ["..."]}`.

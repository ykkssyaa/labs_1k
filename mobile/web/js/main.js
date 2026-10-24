// Точка входа: создание зависимостей (аналог AppScope), маршруты и отрисовка экранов.

import { ApiClient, HttpClient, apiRetryInterceptor, logInterceptor } from './api/api_client.js';
import { KeyValueStorage, LocaleStorage, OnboardingStorage, ThemeModeStorage } from './persistence/key_value_storage.js';
import { openDatabase } from './persistence/database.js';
import { FavoritesDao } from './persistence/favorites_dao.js';
import { HistorySearchDao } from './persistence/history_search_dao.js';
import { FilterSettingsDao } from './persistence/filter_settings_dao.js';
import { DbFilterStorage, LocalFilterStorage } from './persistence/filter_storage.js';
import { PlacesRepository } from './data/places_repository.js';
import { FilterRepository } from './data/filter_repository.js';
import { SearchRepository } from './data/search_repository.js';
import { FavoritesRepository } from './data/favorites_repository.js';
import { LocaleRepository, ReviewsRepository, ThemeModeRepository } from './data/settings_repositories.js';
import { SettingsModel } from './feature/settings/settings_model.js';
import { L10n, resolveLocale } from './l10n/l10n.js';
import { Router } from './navigation/router.js';
import { splashScreen } from './feature/splash/splash_screen.js';
import { onboardingScreen } from './feature/onboarding/onboarding_screen.js';
import { placesScreen } from './feature/places/places_screen.js';
import { placeDetailScreen } from './feature/place_detail/place_detail_screen.js';
import { filterScreen } from './feature/filter/filter_screen.js';
import { searchScreen } from './feature/search/search_screen.js';
import { favoritesScreen } from './feature/favorites/favorites_screen.js';
import { mapScreen } from './feature/map/map_screen.js';
import { settingsScreen } from './feature/settings/settings_screen.js';
import { bottomNav } from './feature/bottom_nav/bottom_nav.js';

// порядок важен: конкретные пути раньше '/places/:id'
const routes = [
  { path: '/splash', screen: splashScreen },
  { path: '/onboarding', screen: onboardingScreen },
  { path: '/places', screen: placesScreen, tab: 'places' },
  { path: '/places/search', screen: searchScreen, tab: 'places', transition: 'search' },
  { path: '/places/filter', screen: filterScreen, tab: 'places' },
  { path: '/places/:id', screen: placeDetailScreen, tab: 'places' },
  { path: '/map', screen: mapScreen, tab: 'map' },
  { path: '/map/:id', screen: placeDetailScreen, tab: 'map' },
  { path: '/favorites', screen: favoritesScreen, tab: 'favorites' },
  { path: '/favorites/:id', screen: placeDetailScreen, tab: 'favorites' },
  { path: '/settings', screen: settingsScreen, tab: 'settings' },
];

async function loadMessages() {
  const load = (lang) => fetch(`js/l10n/${lang}.json`).then((r) => r.json());
  const [ru, en] = await Promise.all([load('ru'), load('en')]);
  return { ru, en };
}

/** Открывает БД; если миграция не удалась — работаем со старой схемой и фильтром в localStorage. */
async function initDatabase(keyValue) {
  try {
    const db = await openDatabase({ keyValue });
    const filterSettingsDao = new FilterSettingsDao(db);
    return { db, filterSettingsDao, filterStorage: new DbFilterStorage(filterSettingsDao) };
  } catch (e) {
    console.error('database v2 is not available, fallback to v1', e);
    const db = await openDatabase({ keyValue, version: 1 });
    return { db, filterSettingsDao: null, filterStorage: new LocalFilterStorage(keyValue) };
  }
}

async function createApp() {
  const keyValue = new KeyValueStorage();
  const http = new HttpClient({ timeout: 30000, interceptors: [logInterceptor(), apiRetryInterceptor()] });
  const api = new ApiClient(http);
  const { db, filterSettingsDao, filterStorage } = await initDatabase(keyValue);

  const favoritesDao = new FavoritesDao(db);
  const historySearchDao = new HistorySearchDao(db);
  await Promise.all([favoritesDao.init(), historySearchDao.init()]);

  const themeModeRepository = new ThemeModeRepository(new ThemeModeStorage(keyValue));
  const localeRepository = new LocaleRepository(new LocaleStorage(keyValue));

  return {
    keyValue,
    l10n: new L10n(await loadMessages()),
    onboardingStorage: new OnboardingStorage(keyValue),
    placesRepository: new PlacesRepository({ api, filterStorage }),
    filterRepository: new FilterRepository({ api, filterStorage, filterSettingsDao }),
    searchRepository: new SearchRepository({ api, historySearchDao }),
    favoritesRepository: new FavoritesRepository({ favoritesDao }),
    reviewsRepository: new ReviewsRepository({ api }),
    themeModeRepository,
    localeRepository,
    settingsModel: new SettingsModel({ themeModeRepository, localeRepository }),
  };
}

function applyTheme(mode) {
  document.documentElement.dataset.theme = mode;
}

function applyLocale(app, choice) {
  app.l10n.locale = resolveLocale(choice, navigator.languages ?? [navigator.language]);
  document.documentElement.lang = app.l10n.locale;
}

async function main() {
  const root = document.getElementById('app');
  const app = await createApp();
  let currentScreen = null;

  applyTheme(app.themeModeRepository.themeModeStream.value);
  app.themeModeRepository.themeModeStream.listen(applyTheme);
  applyLocale(app, app.localeRepository.localeStream.value);
  app.localeRepository.localeStream.listen((choice) => {
    applyLocale(app, choice);
    app.router.refresh();
  });

  function show(match) {
    currentScreen?.dispose?.();
    currentScreen = match.route.screen(app, match);
    const screen = currentScreen.el;
    screen.classList.add('screen');
    root.replaceChildren(screen);
    if (match.route.tab) root.append(bottomNav(app, match.route.tab));
    root.classList.toggle('with-nav', !!match.route.tab);
    window.scrollTo(0, 0);
    currentScreen.onShow?.();
  }

  app.router = new Router(routes, (match, direction) => {
    const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!document.startViewTransition || direction === 'none' || reduceMotion) {
      show(match);
      return;
    }
    document.documentElement.dataset.nav = direction;
    document.startViewTransition(() => show(match));
  });
  app.router.start('/splash');
  window.app = app; // для отладки из консоли
}

main().catch((e) => {
  console.error(e);
  document.getElementById('app').textContent = `Ошибка запуска: ${e.message}`;
});

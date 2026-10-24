// Хранилище «ключ-значение» поверх localStorage — аналог SharedPreferences.
// backing можно подменить (в тестах используется MemoryBacking).

export class MemoryBacking {
  constructor() {
    this.map = new Map();
  }
  getItem(k) {
    return this.map.has(k) ? this.map.get(k) : null;
  }
  setItem(k, v) {
    this.map.set(k, String(v));
  }
  removeItem(k) {
    this.map.delete(k);
  }
}

export class KeyValueStorage {
  constructor(backing = globalThis.localStorage ?? new MemoryBacking()) {
    this.backing = backing;
  }

  getString(key) {
    return this.backing.getItem(key);
  }

  setString(key, value) {
    this.backing.setItem(key, value);
  }

  getJson(key) {
    const raw = this.backing.getItem(key);
    if (raw == null) return null;
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }

  setJson(key, value) {
    this.backing.setItem(key, JSON.stringify(value));
  }

  remove(key) {
    this.backing.removeItem(key);
  }
}

export const StorageKeys = {
  themeMode: 'theme_mode',
  locale: 'locale',
  onboardingDone: 'onboarding_done',
  filter: 'filter',
};

/** Сохранение темы: 'light' | 'dark'. */
export class ThemeModeStorage {
  constructor(storage) {
    this.storage = storage;
  }
  getThemeMode() {
    return this.storage.getString(StorageKeys.themeMode) === 'dark' ? 'dark' : 'light';
  }
  setThemeMode(mode) {
    this.storage.setString(StorageKeys.themeMode, mode);
  }
}

/** Выбранный язык: 'system' | 'ru' | 'en'. */
export class LocaleStorage {
  constructor(storage) {
    this.storage = storage;
  }
  getLocale() {
    return this.storage.getString(StorageKeys.locale) ?? 'system';
  }
  setLocale(locale) {
    this.storage.setString(StorageKeys.locale, locale);
  }
}

/** Признак первого запуска. */
export class OnboardingStorage {
  constructor(storage) {
    this.storage = storage;
  }
  isDone() {
    return this.storage.getString(StorageKeys.onboardingDone) === '1';
  }
  setDone() {
    this.storage.setString(StorageKeys.onboardingDone, '1');
  }
}

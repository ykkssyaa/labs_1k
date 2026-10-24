import { ValueNotifier } from '../core/value_notifier.js';
import { makeApiCall } from '../core/result.js';

/** Тема приложения: значение хранится в ThemeModeStorage, изменения рассылаются подписчикам. */
export class ThemeModeRepository {
  constructor(storage) {
    this.storage = storage;
    this.themeModeStream = new ValueNotifier(storage.getThemeMode());
  }

  setThemeMode(mode) {
    this.storage.setThemeMode(mode);
    this.themeModeStream.value = mode;
  }
}

/** Язык приложения: 'system' | 'ru' | 'en'. */
export class LocaleRepository {
  constructor(storage) {
    this.storage = storage;
    this.localeStream = new ValueNotifier(storage.getLocale());
  }

  setLocale(locale) {
    this.storage.setLocale(locale);
    this.localeStream.value = locale;
  }
}

/** Отзывы о месте. */
export class ReviewsRepository {
  constructor({ api }) {
    this.api = api;
  }

  getReviews(placeId) {
    return makeApiCall(() => this.api.getReviews(placeId));
  }

  addReview(placeId, review) {
    return makeApiCall(() => this.api.addReview(placeId, review));
  }
}

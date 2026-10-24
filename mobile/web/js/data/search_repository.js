import { makeApiCall } from '../core/result.js';
import { Converter, FilterPlacesConverter, PlaceResponseConverter } from '../domain/place.js';

/** Запись истории -> SearchKeywordEntity. */
export class SearchKeywordFromDbConverter extends Converter {
  convert(row) {
    return { id: row.id, keyword: row.keyword };
  }
}

/** Строка запроса -> запись истории. */
export class SearchKeywordToDbConverter extends Converter {
  convert(keyword) {
    return { keyword: keyword.trim() };
  }
}

/** Поиск мест на сервере и история запросов в локальной БД. */
export class SearchRepository {
  constructor({
    api,
    historySearchDao,
    filterConverter = new FilterPlacesConverter(),
    placeConverter = new PlaceResponseConverter(),
    fromDbConverter = new SearchKeywordFromDbConverter(),
    toDbConverter = new SearchKeywordToDbConverter(),
  }) {
    this.api = api;
    this.dao = historySearchDao;
    this.filterConverter = filterConverter;
    this.placeConverter = placeConverter;
    this.fromDbConverter = fromDbConverter;
    this.toDbConverter = toDbConverter;
  }

  /** Текущая история (SearchKeywordEntity[]). */
  get history() {
    return this.fromDbConverter.convertMultiple(this.dao.historySearchStream.value);
  }

  /** Подписка на историю, возвращает отписку. */
  listenHistory(fn) {
    return this.dao.historySearchStream.listen((rows) => fn(this.fromDbConverter.convertMultiple(rows)));
  }

  getFilteredPlaces(filter) {
    return makeApiCall(async () => {
      const data = await this.api.getFilteredPlaces(this.filterConverter.convert(filter));
      return this.placeConverter.convertMultiple(data);
    });
  }

  async saveKeyword(keyword) {
    if (!keyword.trim()) return;
    await this.dao.saveKeyword(this.toDbConverter.convert(keyword));
  }

  async removeKeyword(keyword) {
    await this.dao.removeKeyword(keyword.id);
  }

  async clearHistory() {
    await this.dao.clearHistory();
  }
}

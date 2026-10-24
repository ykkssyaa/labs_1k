import { makeApiCall } from '../core/result.js';
import { FilterPlacesConverter } from '../domain/place.js';

/** Фильтр: подсчёт мест на сервере, сохранение фильтра и пресетов. */
export class FilterRepository {
  constructor({ api, filterStorage, filterSettingsDao = null, filterConverter = new FilterPlacesConverter() }) {
    this.api = api;
    this.filterStorage = filterStorage;
    this.dao = filterSettingsDao;
    this.filterConverter = filterConverter;
  }

  getFilteredPlaces(filter) {
    return makeApiCall(() => this.api.getFilteredPlaces(this.filterConverter.convert(filter)));
  }

  getFilter() {
    return this.filterStorage.getFilter();
  }

  saveFilter(filter) {
    return this.filterStorage.saveFilter(filter);
  }

  clearFilter() {
    return this.filterStorage.clear();
  }

  /** Пресеты доступны только при хранении в БД. */
  get hasPresets() {
    return this.dao != null;
  }

  getPresets() {
    return this.dao ? this.dao.getPresets() : Promise.resolve([]);
  }

  savePreset(name, filter) {
    return this.dao.savePreset(name, filter);
  }

  deletePreset(id) {
    return this.dao.delete(id);
  }
}

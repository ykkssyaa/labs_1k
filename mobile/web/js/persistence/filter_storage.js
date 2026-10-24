import { createDefaultFilter } from '../domain/place.js';
import { StorageKeys } from './key_value_storage.js';

// IFilterStorage: getFilter(): Promise<filter>, saveFilter(filter): Promise, clear(): Promise.

/** Фильтр в localStorage (вариант с SharedPreferences). */
export class LocalFilterStorage {
  constructor(keyValue) {
    this.keyValue = keyValue;
  }

  async getFilter() {
    return { ...createDefaultFilter(), ...this.keyValue.getJson(StorageKeys.filter) };
  }

  async saveFilter(filter) {
    const { types, radius, sortBy } = filter;
    this.keyValue.setJson(StorageKeys.filter, { types, radius, sortBy });
  }

  async clear() {
    this.keyValue.remove(StorageKeys.filter);
  }
}

/** Фильтр в БД через FilterSettingsDao, с кэшем в памяти. */
export class DbFilterStorage {
  constructor(dao) {
    this.dao = dao;
    this.cache = null;
  }

  async getFilter() {
    if (!this.cache) {
      const active = await this.dao.getActive();
      this.cache = active ? pick(active) : createDefaultFilter();
    }
    return { ...this.cache };
  }

  async saveFilter(filter) {
    this.cache = pick(filter);
    await this.dao.setActive(this.cache);
  }

  async clear() {
    this.cache = null;
    await this.dao.deleteActive();
  }
}

const pick = ({ types, radius, sortBy }) => ({ types: [...types], radius, sortBy: sortBy ?? null });

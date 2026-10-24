// Локальная БД на IndexedDB — аналог AppDatabase (Drift).
// Таблицы: favorites, history_search, filter_settings (появилась в версии 2).

import { StorageKeys } from './key_value_storage.js';
import { FilterSettingsConverter } from './filter_settings_dao.js';

export const DB_NAME = 'places_db';
export const SCHEMA_VERSION = 2;

// Миграции по версиям. Всё выполняется в одной versionchange-транзакции:
// если миграция бросит исключение, IndexedDB откатит изменения и версия останется старой.
const migrations = {
  1(db) {
    db.createObjectStore('favorites', { keyPath: 'placeId' }).createIndex('addedAt', 'addedAt');
    db.createObjectStore('history_search', { keyPath: 'id', autoIncrement: true }).createIndex('keyword', 'keyword', {
      unique: true,
    });
  },
  2(db, tx, ctx) {
    const store = db.createObjectStore('filter_settings', { keyPath: 'id', autoIncrement: true });
    store.createIndex('name', 'name');
    store.createIndex('createdAt', 'createdAt');
    store.createIndex('updatedAt', 'updatedAt');
    store.createIndex('isDefault', 'isDefault');

    // перенос фильтра из старого хранилища (localStorage)
    const raw = ctx.keyValue?.getString(StorageKeys.filter);
    if (raw != null) {
      const filter = JSON.parse(raw); // при битых данных — исключение и откат миграции
      const row = new FilterSettingsConverter().convert({ ...filter, name: 'default', isDefault: true });
      tx.objectStore('filter_settings').add(row);
      ctx.migratedFromKeyValue = true;
    }
  },
};

/**
 * Открывает БД и выполняет нужные миграции.
 * @param {{indexedDB?: IDBFactory, version?: number, keyValue?: import('./key_value_storage.js').KeyValueStorage}} options
 */
export function openDatabase({ indexedDB = globalThis.indexedDB, version = SCHEMA_VERSION, keyValue } = {}) {
  const ctx = { keyValue, migratedFromKeyValue: false };
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, version);
    request.onupgradeneeded = (event) => {
      const db = request.result;
      const tx = request.transaction;
      try {
        for (let v = event.oldVersion + 1; v <= version; v++) {
          console.info(`db migration to v${v}`);
          migrations[v](db, tx, ctx);
        }
      } catch (e) {
        console.error('db migration failed, rollback', e);
        tx.abort();
      }
    };
    request.onsuccess = () => {
      // старые данные удаляем только после успешной миграции
      if (ctx.migratedFromKeyValue) keyValue.remove(StorageKeys.filter);
      resolve(request.result);
    };
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error('database is blocked by another tab'));
  });
}

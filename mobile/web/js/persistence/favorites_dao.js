import { ValueNotifier } from '../core/value_notifier.js';
import { req, done } from './idb.js';

const STORE = 'favorites';

/** Работа с таблицей избранного. favoritesStream обновляется после каждого изменения. */
export class FavoritesDao {
  constructor(db) {
    this.db = db;
    this.favoritesStream = new ValueNotifier([]);
  }

  async init() {
    await this._emit();
  }

  /** Все записи, последние добавленные — первыми. */
  async getAll() {
    const rows = await req(this.db.transaction(STORE).objectStore(STORE).index('addedAt').getAll());
    return rows.reverse();
  }

  async add(row) {
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).put(row);
    await done(tx);
    await this._emit();
  }

  async remove(placeId) {
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).delete(placeId);
    await done(tx);
    await this._emit();
  }

  async _emit() {
    this.favoritesStream.value = await this.getAll();
  }
}

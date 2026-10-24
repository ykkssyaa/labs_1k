import { ValueNotifier } from '../core/value_notifier.js';
import { req, done } from './idb.js';

const STORE = 'history_search';
const LIMIT = 10;

/** История поиска. Повторный запрос поднимается наверх, хранится не больше LIMIT записей. */
export class HistorySearchDao {
  constructor(db) {
    this.db = db;
    this.historySearchStream = new ValueNotifier([]);
  }

  async init() {
    await this._emit();
  }

  /** Записи от новых к старым. */
  async getAll() {
    const rows = await req(this.db.transaction(STORE).objectStore(STORE).getAll());
    return rows.reverse();
  }

  async saveKeyword(row) {
    const tx = this.db.transaction(STORE, 'readwrite');
    const store = tx.objectStore(STORE);
    const old = await req(store.index('keyword').getKey(row.keyword));
    if (old != null) store.delete(old);
    store.add(row);
    await done(tx);
    await this._trim();
    await this._emit();
  }

  async removeKeyword(id) {
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).delete(id);
    await done(tx);
    await this._emit();
  }

  async clearHistory() {
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).clear();
    await done(tx);
    await this._emit();
  }

  async _trim() {
    const keys = await req(this.db.transaction(STORE).objectStore(STORE).getAllKeys());
    if (keys.length <= LIMIT) return;
    const tx = this.db.transaction(STORE, 'readwrite');
    keys.slice(0, keys.length - LIMIT).forEach((k) => tx.objectStore(STORE).delete(k));
    await done(tx);
  }

  async _emit() {
    this.historySearchStream.value = await this.getAll();
  }
}

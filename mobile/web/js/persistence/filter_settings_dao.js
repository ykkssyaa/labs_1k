import { Converter, PlaceType } from '../domain/place.js';
import { req, done } from './idb.js';

const STORE = 'filter_settings';

/**
 * Конвертер фильтра в запись таблицы и обратно.
 * Список категорий хранится строкой "park,cafe", радиус «без ограничения» — числом -1,
 * isDefault — 0/1 (булевы значения IndexedDB не индексирует).
 */
export class FilterSettingsConverter extends Converter {
  constructor(now = () => Date.now()) {
    super();
    this.now = now;
  }

  convert(filter) {
    const time = this.now();
    const row = {
      name: filter.name ?? '',
      types: (filter.types ?? []).join(','),
      radius: filter.radius ?? -1,
      sortBy: filter.sortBy ?? '',
      isDefault: filter.isDefault ? 1 : 0,
      createdAt: filter.createdAt ?? time,
      updatedAt: time,
    };
    if (filter.id != null) row.id = filter.id;
    return row;
  }

  convertReverse(row) {
    return {
      id: row.id,
      name: row.name,
      types: row.types ? row.types.split(',').filter((t) => PlaceType.includes(t)) : [],
      radius: row.radius < 0 ? null : row.radius,
      sortBy: row.sortBy || null,
      isDefault: row.isDefault === 1,
      createdAt: row.createdAt,
      updatedAt: row.updatedAt,
    };
  }
}

/** DAO сохранённых фильтров: активный фильтр (isDefault) и пресеты пользователя. */
export class FilterSettingsDao {
  constructor(db, converter = new FilterSettingsConverter()) {
    this.db = db;
    this.converter = converter;
  }

  _store(mode = 'readonly') {
    return this.db.transaction(STORE, mode).objectStore(STORE);
  }

  // CRUD
  async insert(filter) {
    const row = this.converter.convert({ ...filter, id: undefined });
    delete row.id;
    return req(this._store('readwrite').add(row));
  }

  async getById(id) {
    const row = await req(this._store().get(id));
    return row ? this.converter.convertReverse(row) : null;
  }

  async update(filter) {
    const old = await this.getById(filter.id);
    if (!old) throw new Error(`filter ${filter.id} not found`);
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).put(this.converter.convert({ ...old, ...filter, createdAt: old.createdAt }));
    await done(tx);
  }

  async delete(id) {
    const tx = this.db.transaction(STORE, 'readwrite');
    tx.objectStore(STORE).delete(id);
    await done(tx);
  }

  /** Все записи, недавно изменённые — первыми. */
  async getAll() {
    const rows = await req(this._store().index('updatedAt').getAll());
    return rows.reverse().map((r) => this.converter.convertReverse(r));
  }

  // активный фильтр
  async getActive() {
    const row = await req(this._store().index('isDefault').get(1));
    return row ? this.converter.convertReverse(row) : null;
  }

  async setActive(filter) {
    const active = await this.getActive();
    if (active) {
      await this.update({ ...filter, id: active.id, name: 'default', isDefault: true });
    } else {
      await this.insert({ ...filter, name: 'default', isDefault: true });
    }
  }

  async deleteActive() {
    const active = await this.getActive();
    if (active) await this.delete(active.id);
  }

  // пресеты
  async getPresets() {
    return (await this.getAll()).filter((f) => !f.isDefault);
  }

  async savePreset(name, filter) {
    return this.insert({ ...filter, name, isDefault: false });
  }

  /** Поиск пресетов по части имени без учёта регистра. */
  async findByName(query) {
    const q = query.trim().toLowerCase();
    return (await this.getPresets()).filter((f) => f.name.toLowerCase().includes(q));
  }

  /** Записи, созданные в промежутке [from, to] (timestamp, мс). */
  async findByDate(from, to) {
    const rows = await req(this._store().index('createdAt').getAll(IDBKeyRange.bound(from, to)));
    return rows.map((r) => this.converter.convertReverse(r));
  }
}

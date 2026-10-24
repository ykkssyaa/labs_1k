import { FavoritePlaceFromDbConverter, FavoritePlaceToDbConverter } from '../domain/place.js';

/** Избранные места в локальной БД. */
export class FavoritesRepository {
  constructor({ favoritesDao, fromDb = new FavoritePlaceFromDbConverter(), toDb = new FavoritePlaceToDbConverter() }) {
    this.dao = favoritesDao;
    this.fromDb = fromDb;
    this.toDb = toDb;
  }

  get favorites() {
    return this.fromDb.convertMultiple(this.dao.favoritesStream.value);
  }

  listen(fn) {
    return this.dao.favoritesStream.listen((rows) => fn(this.fromDb.convertMultiple(rows)));
  }

  isFavorite(placeId) {
    return this.dao.favoritesStream.value.some((r) => r.placeId === placeId);
  }

  /** Добавляет или убирает место, возвращает новое состояние. */
  async toggle(place) {
    if (this.isFavorite(place.id)) {
      await this.dao.remove(place.id);
      return false;
    }
    await this.dao.add(this.toDb.convert(place));
    return true;
  }
}

import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  FavoritePlaceFromDbConverter,
  FavoritePlaceToDbConverter,
  FilterPlacesConverter,
  PlaceResponseConverter,
  DEFAULT_CENTER,
} from '../../web/js/domain/place.js';
import { FilterSettingsConverter } from '../../web/js/persistence/filter_settings_dao.js';

const place = {
  id: 7,
  name: 'Парк Горького',
  description: 'Парк',
  type: 'park',
  images: ['https://a/1.jpg', 'https://a/2.jpg'],
  lat: 55.73,
  lon: 37.6,
  distance: null,
  rating: null,
  reviewCount: 0,
};

const row = {
  placeId: 7,
  name: 'Парк Горького',
  description: 'Парк',
  type: 2,
  images: 'https://a/1.jpg,https://a/2.jpg',
  lat: 55.73,
  lon: 37.6,
  addedAt: 1000,
};

test('FavoritePlaceToDbConverter: сущность -> запись БД', () => {
  const converter = new FavoritePlaceToDbConverter(() => 1000);
  assert.deepEqual(converter.convert(place), row);
});

test('FavoritePlaceFromDbConverter: запись БД -> сущность', () => {
  assert.deepEqual(new FavoritePlaceFromDbConverter().convert(row), place);
});

test('конвертеры избранного обратимы', () => {
  const back = new FavoritePlaceFromDbConverter().convert(new FavoritePlaceToDbConverter().convert(place));
  assert.deepEqual(back, place);
});

test('FavoritePlaceFromDbConverter: пустые картинки и неизвестный тип', () => {
  const entity = new FavoritePlaceFromDbConverter().convert({ ...row, images: '', type: 99 });
  assert.deepEqual(entity.images, []);
  assert.equal(entity.type, 'other');
});

test('convertMultiple конвертирует список', () => {
  const list = new FavoritePlaceFromDbConverter().convertMultiple([row, { ...row, placeId: 8 }]);
  assert.deepEqual(
    list.map((p) => p.id),
    [7, 8],
  );
});

test('PlaceResponseConverter: dto -> сущность', () => {
  const entity = new PlaceResponseConverter().convert({ id: 1, name: 'A', lat: 1, lng: 2, placeType: 'zoo', description: 'd', urls: ['u'] });
  assert.equal(entity.type, 'other');
  assert.equal(entity.lon, 2);
  assert.equal(entity.rating, null);
});

test('FilterPlacesConverter: радиус добавляет координаты центра', () => {
  const converter = new FilterPlacesConverter();
  assert.deepEqual(converter.convert({ types: ['park'], radius: null }), { typeFilter: ['park'] });
  assert.deepEqual(converter.convert({ types: ['park'], radius: 500, sortBy: 'rating', name: 'парк' }), {
    typeFilter: ['park'],
    lat: DEFAULT_CENTER.lat,
    lng: DEFAULT_CENTER.lng,
    radius: 500,
    nameFilter: 'парк',
    sortBy: 'rating',
  });
});

test('FilterSettingsConverter: список категорий и «любой» радиус', () => {
  const converter = new FilterSettingsConverter(() => 5);
  const dbRow = converter.convert({ name: 'p', types: ['park', 'cafe'], radius: null, sortBy: null, isDefault: true });
  assert.equal(dbRow.types, 'park,cafe');
  assert.equal(dbRow.radius, -1);
  assert.equal(dbRow.isDefault, 1);
  const back = converter.convertReverse({ ...dbRow, id: 3 });
  assert.deepEqual(back.types, ['park', 'cafe']);
  assert.equal(back.radius, null);
  assert.equal(back.isDefault, true);
});

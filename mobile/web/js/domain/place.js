// Сущности и конвертеры, общие для всего приложения.

export const PlaceType = ['temple', 'monument', 'park', 'theatre', 'museum', 'hotel', 'restaurant', 'cafe', 'other'];

export const placeTypeIcon = (type) => `assets/icons/catalog/${type}.svg`;

/** Точка, от которой считается расстояние (центр Москвы, как в исходном проекте). */
export const DEFAULT_CENTER = { lat: 55.754194, lng: 37.620139 };

export const RADIUS_MIN = 100;
export const RADIUS_MAX = 10000;
export const RADIUS_STEP = 100;

/** Фильтр по умолчанию: все категории, без ограничения по расстоянию (radius = null). */
export function createDefaultFilter() {
  return { types: [...PlaceType], radius: null, sortBy: null };
}

/** Базовый конвертер, как Converter<TOut, TIn> во Flutter-проекте. */
export class Converter {
  convert(_input) {
    throw new Error('not implemented');
  }

  convertMultiple(inputs) {
    return inputs.map((i) => this.convert(i));
  }
}

/** PlaceDto (ответ сервера) -> PlaceEntity. */
export class PlaceResponseConverter extends Converter {
  convert(dto) {
    return {
      id: dto.id,
      name: dto.name,
      description: dto.description,
      type: PlaceType.includes(dto.placeType) ? dto.placeType : 'other',
      images: dto.urls ?? [],
      lat: dto.lat,
      lon: dto.lng,
      distance: dto.distance ?? null,
      rating: dto.rating ?? null,
      reviewCount: dto.reviewCount ?? 0,
    };
  }
}

/** PlaceEntity -> запись таблицы избранного. Картинки хранятся строкой через запятую. */
export class FavoritePlaceToDbConverter extends Converter {
  constructor(now = () => Date.now()) {
    super();
    this.now = now;
  }

  convert(place) {
    return {
      placeId: place.id,
      name: place.name,
      description: place.description,
      type: PlaceType.indexOf(place.type),
      images: place.images.join(','),
      lat: place.lat,
      lon: place.lon,
      addedAt: this.now(),
    };
  }
}

/** Запись таблицы избранного -> PlaceEntity. */
export class FavoritePlaceFromDbConverter extends Converter {
  convert(row) {
    return {
      id: row.placeId,
      name: row.name,
      description: row.description,
      type: PlaceType[row.type] ?? 'other',
      images: row.images ? row.images.split(',') : [],
      lat: row.lat,
      lon: row.lon,
      distance: null,
      rating: null,
      reviewCount: 0,
    };
  }
}

/** Фильтр приложения -> FilterPlacesRequestDto. */
export class FilterPlacesConverter extends Converter {
  convert(filter) {
    const dto = { typeFilter: filter.types };
    if (filter.radius != null) {
      dto.lat = DEFAULT_CENTER.lat;
      dto.lng = DEFAULT_CENTER.lng;
      dto.radius = filter.radius;
    }
    if (filter.name) dto.nameFilter = filter.name;
    if (filter.sortBy) dto.sortBy = filter.sortBy;
    return dto;
  }
}

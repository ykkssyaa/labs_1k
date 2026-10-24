import { makeApiCall } from '../core/result.js';
import { FilterPlacesConverter, PlaceResponseConverter } from '../domain/place.js';

/** Места с сервера. Список запрашивается с учётом сохранённого фильтра. */
export class PlacesRepository {
  constructor({ api, filterStorage, placeConverter = new PlaceResponseConverter(), filterConverter = new FilterPlacesConverter() }) {
    this.api = api;
    this.filterStorage = filterStorage;
    this.placeConverter = placeConverter;
    this.filterConverter = filterConverter;
  }

  getPlaces() {
    return makeApiCall(async () => {
      const filter = await this.filterStorage.getFilter();
      const data = await this.api.getFilteredPlaces(this.filterConverter.convert(filter));
      return this.placeConverter.convertMultiple(data);
    });
  }

  /** Все места без фильтра (для карты). */
  getAllPlaces() {
    return makeApiCall(async () => this.placeConverter.convertMultiple(await this.api.getPlaces()));
  }

  getPlaceDetails(id) {
    return makeApiCall(async () => this.placeConverter.convert(await this.api.getPlaceDetails(id)));
  }
}

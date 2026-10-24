import { HttpError, TimeoutError } from '../core/failures.js';

export const ApiUrls = {
  places: '/place',
  placeDetails: (id) => `/place/${id}`,
  filteredPlaces: '/filtered_places',
  reviews: (id) => `/place/${id}/reviews`,
};

/**
 * HTTP-клиент на fetch (аналог Dio из AppDioConfigurator):
 * базовый URL, таймаут 30 сек и цепочка перехватчиков.
 */
export class HttpClient {
  constructor({ baseUrl = '', timeout = 30000, interceptors = [], fetchFn = (...args) => globalThis.fetch(...args) } = {}) {
    this.baseUrl = baseUrl;
    this.timeout = timeout;
    this.interceptors = interceptors;
    // fetch в браузере нельзя вызывать как метод другого объекта, поэтому обёртка
    this.fetchFn = fetchFn;
  }

  request(method, path, body) {
    const send = () => this._send(method, path, body);
    // каждый перехватчик оборачивает следующий
    const chain = this.interceptors.reduceRight((next, interceptor) => () => interceptor(next), send);
    return chain();
  }

  async _send(method, path, body) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);
    try {
      const response = await this.fetchFn(this.baseUrl + path, {
        method,
        headers: body ? { 'Content-Type': 'application/json' } : {},
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });
      const data = await response.json().catch(() => null);
      if (!response.ok) throw new HttpError(response.status, data);
      return data;
    } catch (e) {
      if (e.name === 'AbortError') throw new TimeoutError(`timeout ${this.timeout} ms`);
      throw e;
    } finally {
      clearTimeout(timer);
    }
  }
}

/** Повторяет запрос при сетевых ошибках и 5xx/429 с экспоненциальной задержкой. */
export function apiRetryInterceptor({ retries = 3, baseDelay = 500, sleep = defaultSleep } = {}) {
  return async (next) => {
    for (let attempt = 0; ; attempt++) {
      try {
        return await next();
      } catch (e) {
        if (attempt >= retries || !isRetryable(e)) throw e;
        const delay = baseDelay * 2 ** attempt;
        console.warn(`retry #${attempt + 1} in ${delay} ms:`, e.message);
        await sleep(delay);
      }
    }
  };
}

function isRetryable(e) {
  if (e instanceof HttpError) return e.status >= 500 || e.status === 429;
  return e instanceof TypeError || e instanceof TimeoutError;
}

const defaultSleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Пишет запросы в консоль. */
export function logInterceptor() {
  return async (next) => {
    const start = Date.now();
    try {
      return await next();
    } finally {
      console.debug(`request done in ${Date.now() - start} ms`);
    }
  };
}

/** Методы API приложения. */
export class ApiClient {
  constructor(http) {
    this.http = http;
  }

  getPlaces() {
    return this.http.request('GET', ApiUrls.places);
  }

  getPlaceDetails(id) {
    return this.http.request('GET', ApiUrls.placeDetails(id));
  }

  /** @param {object} filter FilterPlacesRequestDto */
  getFilteredPlaces(filter) {
    return this.http.request('POST', ApiUrls.filteredPlaces, filter);
  }

  getReviews(placeId) {
    return this.http.request('GET', ApiUrls.reviews(placeId));
  }

  /** @param {{author: string, rating: number, text: string}} review */
  addReview(placeId, review) {
    return this.http.request('POST', ApiUrls.reviews(placeId), review);
  }
}

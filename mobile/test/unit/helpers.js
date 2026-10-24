import { readFileSync } from 'node:fs';
import { L10n } from '../../web/js/l10n/l10n.js';

const arb = (lang) => JSON.parse(readFileSync(new URL(`../../web/js/l10n/${lang}.json`, import.meta.url)));

export function createL10n(locale = 'ru') {
  return new L10n({ ru: arb('ru'), en: arb('en') }, locale);
}

/** Простой «мок»: запоминает вызовы и возвращает заданный результат. */
export function fn(impl = () => undefined) {
  const mock = (...args) => {
    mock.calls.push(args);
    return impl(...args);
  };
  mock.calls = [];
  return mock;
}

export const placeDto = {
  id: 1,
  name: 'Государственный Эрмитаж',
  lat: 59.9452,
  lng: 30.3245,
  placeType: 'museum',
  description: 'Музей',
  urls: ['https://a/1.jpg', 'https://a/2.jpg'],
  rating: 4.5,
  reviewCount: 2,
};

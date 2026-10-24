import { test } from 'node:test';
import assert from 'node:assert/strict';
import { HttpClient, apiRetryInterceptor } from '../../web/js/api/api_client.js';
import {
  BadRequestFailure,
  HttpError,
  ServerNotRespondingFailure,
  TimeoutError,
  TooManyRequestsFailure,
  UnauthorizedFailure,
  unwrapError,
} from '../../web/js/core/failures.js';
import { resolveLocale } from '../../web/js/l10n/l10n.js';
import { createL10n, fn } from './helpers.js';

test('unwrapError: HTTP-коды -> типы ошибок', () => {
  assert.ok(unwrapError(new HttpError(400)) instanceof BadRequestFailure);
  assert.ok(unwrapError(new HttpError(401)) instanceof UnauthorizedFailure);
  assert.ok(unwrapError(new HttpError(429)) instanceof TooManyRequestsFailure);
  assert.ok(unwrapError(new HttpError(504)) instanceof ServerNotRespondingFailure);
  assert.ok(unwrapError(new TimeoutError()) instanceof ServerNotRespondingFailure);
});

test('apiRetryInterceptor повторяет 503 с экспоненциальной задержкой', async () => {
  const sleep = fn(async () => {});
  let calls = 0;
  const fetchFn = async () => {
    calls++;
    return calls < 3 ? new Response('{}', { status: 503 }) : Response.json([1, 2]);
  };
  const http = new HttpClient({ fetchFn, interceptors: [apiRetryInterceptor({ sleep })] });
  assert.deepEqual(await http.request('GET', '/place'), [1, 2]);
  assert.equal(calls, 3);
  assert.deepEqual(
    sleep.calls.map((c) => c[0]),
    [500, 1000],
  );
});

test('apiRetryInterceptor не повторяет 400', async () => {
  let calls = 0;
  const fetchFn = async () => (calls++, new Response('{"error":"bad"}', { status: 400 }));
  const http = new HttpClient({ fetchFn, interceptors: [apiRetryInterceptor({ sleep: async () => {} })] });
  await assert.rejects(http.request('GET', '/place'), (e) => e.status === 400 && e.body.error === 'bad');
  assert.equal(calls, 1);
});

test('множественное число по-русски', () => {
  const l10n = createL10n('ru');
  assert.equal(l10n.t('places.found', { count: 1 }), 'Найдено 1 место');
  assert.equal(l10n.t('places.found', { count: 3 }), 'Найдено 3 места');
  assert.equal(l10n.t('places.found', { count: 11 }), 'Найдено 11 мест');
  assert.equal(l10n.t('places.found', { count: 0 }), 'Мест не найдено');
  assert.equal(l10n.t('reviews.count', { count: 22 }), '22 отзыва');
});

test('числа, валюта и даты зависят от языка', () => {
  const ru = createL10n('ru');
  const en = createL10n('en');
  assert.match(ru.t('settings.avgPrice', { price: 1500.5 }), /1\s500,50\s₽/);
  assert.match(en.t('settings.avgPrice', { price: 1500.5 }), /RUB\s1,500\.50/);
  assert.equal(ru.t('settings.updatedAt', { date: '2026-10-24' }), 'Каталог обновлён 24 октября 2026 г.');
  assert.equal(en.t('settings.updatedAt', { date: '2026-10-24' }), 'Catalog updated on October 24, 2026');
});

test('язык: выбор пользователя, системный, иначе русский', () => {
  assert.equal(resolveLocale('en', ['ru-RU']), 'en');
  assert.equal(resolveLocale('system', ['en-US', 'ru']), 'en');
  assert.equal(resolveLocale('system', ['de-DE']), 'ru');
});

test('у всех ключей ARB есть описание, языки совпадают', async () => {
  const { readFileSync } = await import('node:fs');
  const load = (l) => JSON.parse(readFileSync(new URL(`../../web/js/l10n/${l}.json`, import.meta.url)));
  const ru = load('ru');
  const en = load('en');
  for (const [feature, strings] of Object.entries(ru)) {
    if (feature.startsWith('@')) continue;
    for (const key of Object.keys(strings).filter((k) => !k.startsWith('@'))) {
      assert.ok(strings[`@${key}`]?.description, `${feature}.${key} без описания`);
      assert.equal(typeof en[feature]?.[key], 'string', `${feature}.${key} нет в en.json`);
    }
  }
});

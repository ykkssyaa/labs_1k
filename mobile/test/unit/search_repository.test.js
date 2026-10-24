import { test } from 'node:test';
import assert from 'node:assert/strict';
import { SearchRepository } from '../../web/js/data/search_repository.js';
import { ValueNotifier } from '../../web/js/core/value_notifier.js';
import { HttpError, InternalServerFailure, NoNetworkFailure } from '../../web/js/core/failures.js';
import { fn, placeDto } from './helpers.js';

function createRepository({ getFilteredPlaces = async () => [placeDto] } = {}) {
  const api = { getFilteredPlaces: fn(getFilteredPlaces) };
  const dao = {
    historySearchStream: new ValueNotifier([{ id: 1, keyword: 'парк' }]),
    saveKeyword: fn(async () => {}),
    removeKeyword: fn(async () => {}),
    clearHistory: fn(async () => {}),
  };
  return { repo: new SearchRepository({ api, historySearchDao: dao }), api, dao };
}

test('getFilteredPlaces отправляет фильтр с названием и конвертирует ответ', async () => {
  const { repo, api } = createRepository();
  const result = await repo.getFilteredPlaces({ types: ['museum'], radius: null, name: 'эрмитаж' });
  assert.equal(result.ok, true);
  assert.equal(result.data[0].name, 'Государственный Эрмитаж');
  assert.equal(result.data[0].lon, 30.3245);
  assert.deepEqual(api.getFilteredPlaces.calls[0][0], { typeFilter: ['museum'], nameFilter: 'эрмитаж' });
});

test('getFilteredPlaces: ошибка сервера -> InternalServerFailure', async () => {
  const { repo } = createRepository({
    getFilteredPlaces: async () => {
      throw new HttpError(500, { error: 'boom' });
    },
  });
  const result = await repo.getFilteredPlaces({ types: [], radius: null });
  assert.equal(result.ok, false);
  assert.ok(result.error instanceof InternalServerFailure);
});

test('getFilteredPlaces: нет сети -> NoNetworkFailure', async () => {
  const { repo } = createRepository({
    getFilteredPlaces: async () => {
      throw new TypeError('Failed to fetch');
    },
  });
  const result = await repo.getFilteredPlaces({ types: [], radius: null });
  assert.ok(result.error instanceof NoNetworkFailure);
});

test('saveKeyword сохраняет обрезанную строку и пропускает пустую', async () => {
  const { repo, dao } = createRepository();
  await repo.saveKeyword('  музей ');
  await repo.saveKeyword('   ');
  assert.equal(dao.saveKeyword.calls.length, 1);
  assert.deepEqual(dao.saveKeyword.calls[0][0], { keyword: 'музей' });
});

test('removeKeyword и clearHistory вызывают DAO', async () => {
  const { repo, dao } = createRepository();
  await repo.removeKeyword({ id: 1, keyword: 'парк' });
  await repo.clearHistory();
  assert.deepEqual(dao.removeKeyword.calls[0], [1]);
  assert.equal(dao.clearHistory.calls.length, 1);
});

test('история приходит в виде SearchKeywordEntity и обновляется подпиской', () => {
  const { repo, dao } = createRepository();
  assert.deepEqual(repo.history, [{ id: 1, keyword: 'парк' }]);
  const received = [];
  const unlisten = repo.listenHistory((h) => received.push(h));
  dao.historySearchStream.value = [{ id: 2, keyword: 'кафе' }];
  unlisten();
  dao.historySearchStream.value = [];
  assert.deepEqual(received, [[{ id: 2, keyword: 'кафе' }]]);
});

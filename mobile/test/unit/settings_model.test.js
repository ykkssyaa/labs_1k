import { test } from 'node:test';
import assert from 'node:assert/strict';
import { SettingsModel } from '../../web/js/feature/settings/settings_model.js';
import { LocaleRepository, ThemeModeRepository } from '../../web/js/data/settings_repositories.js';
import { KeyValueStorage, LocaleStorage, MemoryBacking, ThemeModeStorage } from '../../web/js/persistence/key_value_storage.js';

function createModel(backing = new MemoryBacking()) {
  const keyValue = new KeyValueStorage(backing);
  const themeModeRepository = new ThemeModeRepository(new ThemeModeStorage(keyValue));
  const localeRepository = new LocaleRepository(new LocaleStorage(keyValue));
  return { model: new SettingsModel({ themeModeRepository, localeRepository }), themeModeRepository, backing };
}

test('по умолчанию светлая тема и системный язык', () => {
  const { model } = createModel();
  assert.equal(model.currentThemeMode, 'light');
  assert.equal(model.currentLocale, 'system');
});

test('setThemeMode меняет тему и оповещает подписчиков', () => {
  const { model, themeModeRepository } = createModel();
  const received = [];
  themeModeRepository.themeModeStream.listen((m) => received.push(m));
  model.setThemeMode('dark');
  assert.equal(model.currentThemeMode, 'dark');
  assert.deepEqual(received, ['dark']);
});

test('тема и язык сохраняются между «сессиями»', () => {
  const backing = new MemoryBacking();
  createModel(backing).model.setThemeMode('dark');
  createModel(backing).model.setLocale('en');
  const { model } = createModel(backing);
  assert.equal(model.currentThemeMode, 'dark');
  assert.equal(model.currentLocale, 'en');
});

test('неизвестная тема — ошибка', () => {
  const { model } = createModel();
  assert.throws(() => model.setThemeMode('pink'));
});

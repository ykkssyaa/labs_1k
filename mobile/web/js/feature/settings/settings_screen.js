import { h, icon } from '../../uikit/dom.js';
import { showToast } from '../../uikit/widgets.js';

const LANGUAGES = [
  ['system', 'settings.languageSystem'],
  ['ru', 'settings.languageRu'],
  ['en', 'settings.languageEn'],
];

// условные значения для примеров форматирования валюты и даты
const AVG_PRICE = 1500.5;
const CATALOG_DATE = '2026-10-24';

/** Настройки: тема, язык, туториал, сброс фильтров, примеры форматирования. */
export function settingsScreen(app) {
  const { l10n, settingsModel: model, router } = app;

  const themeSwitch = h('input', {
    type: 'checkbox',
    class: 'switch',
    role: 'switch',
    checked: model.currentThemeMode === 'dark',
    'data-testid': 'theme-switch',
    onChange: (e) => model.setThemeMode(e.target.checked ? 'dark' : 'light'),
  });

  const language = h(
    'select',
    { class: 'select', 'data-testid': 'language-select', onChange: (e) => model.setLocale(e.target.value) },
    LANGUAGES.map(([value, key]) => h('option', { value, selected: model.currentLocale === value }, l10n.t(key))),
  );

  const placesCount = h('span', {}, '…');
  app.placesRepository.getAllPlaces().then((r) => {
    if (r.ok) placesCount.textContent = l10n.t('settings.placesInDb', { count: r.data.length });
  });

  async function clearFilters() {
    const repo = app.filterRepository;
    await repo.clearFilter();
    for (const p of await repo.getPresets()) await repo.deletePreset(p.id);
    showToast(l10n.t('settings.filtersCleared'));
  }

  const el = h(
    'div',
    { class: 'screen settings' },
    h('header', { class: 'app-bar app-bar--center' }, h('h1', { class: 'app-bar__title' }, l10n.t('settings.title'))),
    h(
      'ul',
      { class: 'settings__list' },
      h('li', {}, h('label', { class: 'settings__row' }, h('span', {}, l10n.t('settings.darkTheme')), themeSwitch)),
      h('li', {}, h('label', { class: 'settings__row' }, h('span', {}, l10n.t('settings.language')), language)),
      h(
        'li',
        {},
        h('button', { class: 'settings__row', onClick: () => router.go('/onboarding?from=settings') }, h('span', {}, l10n.t('settings.tutorial')), icon('info', 'accent')),
      ),
      h('li', {}, h('button', { class: 'settings__row', 'data-testid': 'clear-filters', onClick: clearFilters }, h('span', {}, l10n.t('settings.clearFilters')), icon('bucket', 'accent'))),
    ),
    h('h2', { class: 'section-title' }, l10n.t('settings.about')),
    h(
      'ul',
      { class: 'settings__about' },
      h('li', {}, placesCount),
      h('li', {}, l10n.t('settings.avgPrice', { price: AVG_PRICE })),
      h('li', {}, l10n.t('settings.updatedAt', { date: CATALOG_DATE })),
    ),
  );
  return { el };
}

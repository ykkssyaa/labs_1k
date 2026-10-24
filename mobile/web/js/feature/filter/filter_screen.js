import { debounce, h, icon, render } from '../../uikit/dom.js';
import { backButton, appBar, categoryIcon, loader, showToast } from '../../uikit/widgets.js';
import { createDefaultFilter, PlaceType, RADIUS_MAX, RADIUS_MIN, RADIUS_STEP } from '../../domain/place.js';

const SORTS = [
  [null, 'filter.sortNone'],
  ['rating', 'filter.sortRating'],
  ['distance', 'filter.sortDistance'],
];

/** Фильтр: категории, радиус, сортировка, пресеты. Количество мест считает сервер. */
export function filterScreen(app) {
  const { l10n, filterRepository: repo } = app;
  let filter = createDefaultFilter();
  let countFailed = false;

  const categories = h('div', { class: 'categories' });
  const radiusLabel = h('span', { class: 'filter__radius-value' });
  // крайнее правое положение слайдера — «любое расстояние»
  const slider = h('input', {
    type: 'range',
    class: 'slider',
    min: RADIUS_MIN,
    max: RADIUS_MAX + RADIUS_STEP,
    step: RADIUS_STEP,
    'aria-label': l10n.t('filter.distance'),
    onInput: () => {
      const v = Number(slider.value);
      filter.radius = v > RADIUS_MAX ? null : v;
      update();
    },
  });
  const sorts = h('div', { class: 'segmented' });
  const presets = h('div', { class: 'presets' });
  const showButton = h('button', { class: 'main-btn', 'data-testid': 'show-button', onClick: apply });

  function update() {
    render(
      categories,
      PlaceType.map((type) => {
        const selected = filter.types.includes(type);
        return h(
          'button',
          { class: `category ${selected ? 'selected' : ''}`, 'aria-pressed': String(selected), onClick: () => toggleType(type) },
          h('span', { class: 'category__circle' }, categoryIcon(type), selected && h('span', { class: 'category__tick' }, icon('tick'))),
          h('span', { class: 'category__label' }, l10n.t(`placeType.${type}`)),
        );
      }),
    );
    slider.value = filter.radius ?? RADIUS_MAX + RADIUS_STEP;
    radiusLabel.textContent =
      filter.radius == null ? l10n.t('filter.distanceAny') : l10n.t('filter.distanceUpTo', { distance: l10n.distance(filter.radius) });
    render(
      sorts,
      SORTS.map(([value, key]) =>
        h('button', { class: filter.sortBy === value ? 'active' : '', 'aria-pressed': String(filter.sortBy === value), onClick: () => setSort(value) }, l10n.t(key)),
      ),
    );
    countPlaces();
  }

  function toggleType(type) {
    filter.types = filter.types.includes(type) ? filter.types.filter((t) => t !== type) : [...filter.types, type];
    update();
  }

  function setSort(value) {
    filter.sortBy = value;
    update();
  }

  const countPlaces = debounce(async () => {
    if (filter.types.length === 0) {
      showCount(0);
      return;
    }
    countFailed = false;
    render(showButton, loader('small'));
    showButton.disabled = true;
    const requested = JSON.stringify(filter);
    const result = await repo.getFilteredPlaces(filter);
    if (requested !== JSON.stringify(filter)) return; // фильтр уже поменялся
    if (result.ok) showCount(result.data.length);
    else {
      countFailed = true;
      showButton.disabled = false;
      render(showButton, icon('refresh'), l10n.t('errors.retry'));
    }
  }, 300);

  function showCount(count) {
    showButton.disabled = count === 0;
    render(showButton, l10n.t('filter.show', { count }));
  }

  async function apply() {
    if (countFailed) {
      countPlaces();
      return;
    }
    await repo.saveFilter(filter);
    app.router.back();
  }

  async function loadPresets() {
    if (!repo.hasPresets) return;
    const list = await repo.getPresets();
    render(
      presets,
      list.length === 0 && h('p', { class: 'hint' }, l10n.t('filter.noPresets')),
      list.map((p) =>
        h(
          'span',
          { class: 'chip' },
          h('button', { class: 'chip__label', onClick: () => ((filter = { types: p.types, radius: p.radius, sortBy: p.sortBy }), update()) }, p.name),
          h('button', { class: 'chip__remove', 'aria-label': l10n.t('common.remove'), onClick: async () => (await repo.deletePreset(p.id), loadPresets()) }, icon('close')),
        ),
      ),
      h('button', { class: 'text-btn', onClick: savePreset }, icon('plus'), l10n.t('filter.savePreset')),
    );
  }

  async function savePreset() {
    const name = prompt(l10n.t('filter.presetName'))?.trim();
    if (!name) return;
    await repo.savePreset(name, filter);
    showToast(name);
    loadPresets();
  }

  const el = h(
    'div',
    { class: 'screen filter' },
    appBar({
      leading: backButton(app),
      title: '',
      actions: [h('button', { class: 'text-btn accent', onClick: () => ((filter = createDefaultFilter()), update()) }, l10n.t('filter.clear'))],
    }),
    h('h2', { class: 'section-title' }, l10n.t('filter.categories')),
    categories,
    h('div', { class: 'filter__radius' }, h('h2', { class: 'section-title' }, l10n.t('filter.distance')), radiusLabel),
    slider,
    h('h2', { class: 'section-title' }, l10n.t('filter.sort')),
    sorts,
    repo.hasPresets && h('h2', { class: 'section-title' }, l10n.t('filter.presets')),
    presets,
    h('div', { class: 'bottom-action' }, showButton),
  );

  repo.getFilter().then((saved) => {
    filter = saved;
    update();
  });
  loadPresets();
  return { el };
}

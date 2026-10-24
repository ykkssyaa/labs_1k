import { h, icon, render } from '../../uikit/dom.js';
import { errorPlaceholder, emptyState, placeCard, skeletonCard } from '../../uikit/widgets.js';

const PULL_THRESHOLD = 70;

/** Список мест: поиск, фильтр, обновление потягиванием вниз, скелетоны при загрузке. */
export function placesScreen(app) {
  const { l10n, router } = app;

  const searchBar = h(
    'div',
    { class: 'search-bar' },
    h(
      'button',
      { class: 'search-bar__field', 'data-testid': 'open-search', onClick: () => router.go('/places/search') },
      icon('search'),
      h('span', {}, l10n.t('common.search')),
    ),
    h('button', { class: 'icon-btn search-bar__filter', 'aria-label': l10n.t('filter.categories'), 'data-testid': 'open-filter', onClick: () => router.go('/places/filter') }, icon('filter')),
  );
  const counter = h('p', { class: 'places__counter' });
  const pullHint = h('div', { class: 'pull-hint' }, icon('refresh'));
  const list = h('div', { class: 'places__list', 'data-testid': 'places-list' });
  const refreshBtn = h('button', { class: 'icon-btn', 'aria-label': l10n.t('places.refresh'), onClick: () => load() }, icon('refresh'));

  const el = h(
    'div',
    { class: 'screen places' },
    h('header', { class: 'large-title' }, h('h1', {}, l10n.t('places.title')), refreshBtn),
    h('div', { class: 'small-title' }, l10n.t('places.titleSmall')),
    searchBar,
    pullHint,
    counter,
    list,
  );

  async function load() {
    counter.textContent = '';
    render(list, Array.from({ length: 4 }, skeletonCard));
    const result = await app.placesRepository.getPlaces();
    if (!result.ok) {
      render(list, errorPlaceholder(app, result.error, load));
      return;
    }
    const places = result.data;
    counter.textContent = l10n.t('places.found', { count: places.length });
    if (places.length === 0) {
      render(list, emptyState({ iconName: 'error_search', title: l10n.t('common.emptyTitle'), description: l10n.t('common.emptyDescription') }));
      return;
    }
    render(list, places.map((p) => placeCard(app, p, { onPressed: (place) => router.go(`/places/${place.id}`, place) })));
  }

  setupPullToRefresh(el, pullHint, load);
  load();
  return { el };
}

/** Аналог RefreshIndicator: потянуть список вниз, находясь в самом верху. */
function setupPullToRefresh(el, hint, onRefresh) {
  let startY = null;
  let pulled = 0;
  el.addEventListener('touchstart', (e) => {
    startY = document.scrollingElement.scrollTop === 0 ? e.touches[0].clientY : null;
  });
  el.addEventListener('touchmove', (e) => {
    if (startY == null) return;
    pulled = Math.max(0, e.touches[0].clientY - startY);
    hint.style.height = `${Math.min(pulled, PULL_THRESHOLD * 1.3)}px`;
    hint.classList.toggle('ready', pulled > PULL_THRESHOLD);
  });
  el.addEventListener('touchend', () => {
    if (startY != null && pulled > PULL_THRESHOLD) onRefresh();
    startY = null;
    pulled = 0;
    hint.style.height = '0';
    hint.classList.remove('ready');
  });
}

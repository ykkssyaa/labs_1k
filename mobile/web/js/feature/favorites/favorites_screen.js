import { h, render } from '../../uikit/dom.js';
import { emptyState, placeCard } from '../../uikit/widgets.js';

/** Избранные места из локальной БД. */
export function favoritesScreen(app) {
  const { l10n, favoritesRepository: repo, router } = app;
  const counter = h('p', { class: 'places__counter' });
  const list = h('div', { class: 'places__list' });

  function show(places) {
    counter.textContent = places.length ? l10n.t('favorites.count', { count: places.length }) : '';
    if (places.length === 0) {
      render(list, emptyState({ iconName: 'card', title: l10n.t('favorites.emptyTitle'), description: l10n.t('favorites.emptyDescription') }));
      return;
    }
    render(list, places.map((p) => placeCard(app, p, { onPressed: (place) => router.go(`/favorites/${place.id}`, place) })));
  }

  const unlisten = repo.listen(show);
  show(repo.favorites);
  const el = h('div', { class: 'screen favorites' }, h('header', { class: 'app-bar app-bar--center' }, h('h1', { class: 'app-bar__title' }, l10n.t('favorites.title'))), counter, list);
  return { el, dispose: unlisten };
}

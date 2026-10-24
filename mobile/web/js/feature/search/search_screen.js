import { debounce, h, icon, render } from '../../uikit/dom.js';
import { appBar, backButton, emptyState, errorPlaceholder, loader, networkImage } from '../../uikit/widgets.js';

/** Поиск мест по названию с учётом сохранённого фильтра. Пустой запрос — история поиска. */
export function searchScreen(app) {
  const { l10n, searchRepository: repo, router } = app;

  const input = h('input', {
    class: 'search-bar__input',
    type: 'search',
    placeholder: l10n.t('common.search'),
    'data-testid': 'search-input',
    autocomplete: 'off',
  });
  const clear = h('button', { class: 'icon-btn search-bar__clear', 'aria-label': l10n.t('common.clear'), onClick: () => ((input.value = ''), onQuery()) }, icon('clear'));
  const body = h('div', { class: 'search__body' });

  const onQuery = () => {
    const query = input.value;
    console.log('search query:', query); // ЛР1, задание 2: вывод ввода в консоль
    clear.style.visibility = query ? 'visible' : 'hidden';
    if (query.trim()) {
      render(body, h('div', { class: 'center' }, loader()));
      search(query.trim());
    } else {
      showHistory();
    }
  };

  const search = debounce(async (query) => {
    const filter = await app.filterRepository.getFilter();
    const result = await repo.getFilteredPlaces({ ...filter, name: query });
    if (input.value.trim() !== query) return;
    if (!result.ok) {
      render(body, errorPlaceholder(app, result.error, onQuery));
      return;
    }
    if (result.data.length === 0) {
      render(body, emptyState({ iconName: 'error_search', title: l10n.t('common.emptyTitle'), description: l10n.t('common.emptyDescription') }));
      return;
    }
    render(body, h('ul', { class: 'search__results' }, result.data.map((p) => resultItem(p, query))));
  }, 300);

  function resultItem(place, query) {
    return h(
      'li',
      {},
      h(
        'button',
        {
          class: 'search-result',
          onClick: () => {
            repo.saveKeyword(input.value);
            router.go(`/places/${place.id}`, place);
          },
        },
        h('span', { class: 'search-result__image' }, networkImage(place.images[0], '', place.type)),
        h('span', { class: 'search-result__text' }, h('span', { class: 'search-result__name' }, highlight(place.name, query)), h('span', { class: 'search-result__type' }, l10n.t(`placeType.${place.type}`))),
      ),
    );
  }

  function showHistory() {
    if (input.value.trim()) return;
    const history = repo.history;
    if (history.length === 0) {
      render(body);
      return;
    }
    render(
      body,
      h('h2', { class: 'section-title' }, l10n.t('search.historyTitle'), h('span', { class: 'hint' }, ` · ${l10n.t('search.historyCount', { count: history.length })}`)),
      h(
        'ul',
        { class: 'history' },
        history.map((k) =>
          h(
            'li',
            {},
            h('button', { class: 'history__keyword', onClick: () => ((input.value = k.keyword), onQuery()) }, k.keyword),
            h('button', { class: 'icon-btn', 'aria-label': l10n.t('common.remove'), onClick: () => repo.removeKeyword(k) }, icon('close')),
          ),
        ),
      ),
      h('button', { class: 'text-btn accent', onClick: () => repo.clearHistory() }, l10n.t('search.clearHistory')),
    );
  }

  input.addEventListener('input', onQuery);
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') repo.saveKeyword(input.value);
  });
  const unlisten = repo.listenHistory(showHistory);
  onQuery();

  const el = h(
    'div',
    { class: 'screen search' },
    appBar({ leading: backButton(app), title: l10n.t('search.title') }),
    h('div', { class: 'search-bar search-bar--active' }, icon('search'), input, clear),
    body,
  );
  return { el, dispose: unlisten, onShow: () => input.focus() };
}

/** Выделяет совпадение с запросом жирным. */
function highlight(name, query) {
  const i = name.toLowerCase().indexOf(query.toLowerCase());
  if (i < 0) return name;
  return [name.slice(0, i), h('b', {}, name.slice(i, i + query.length)), name.slice(i + query.length)];
}

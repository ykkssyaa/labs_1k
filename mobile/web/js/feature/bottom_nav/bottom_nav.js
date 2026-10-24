import { h, icon } from '../../uikit/dom.js';
import { TABS } from '../../navigation/router.js';

const ICONS = {
  places: ['list', 'list_full'],
  map: ['map', 'map_full'],
  favorites: ['heart', 'heart_full'],
  settings: ['settings', 'settings_full'],
};

/** Нижняя панель навигации. */
export function bottomNav(app, activeTab) {
  return h(
    'nav',
    { class: 'bottom-nav', 'data-testid': 'bottom-nav' },
    TABS.map((tab) => {
      const active = tab === activeTab;
      return h(
        'button',
        {
          class: `bottom-nav__item ${active ? 'active' : ''}`,
          'aria-label': app.l10n.t(`nav.${tab}`),
          'aria-current': active ? 'page' : null,
          dataset: { tab },
          onClick: () => app.router.openTab(tab),
        },
        icon(ICONS[tab][active ? 1 : 0]),
      );
    }),
  );
}

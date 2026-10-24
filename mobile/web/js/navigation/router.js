// Навигация на hash-адресах (#/places/3).
// У каждой вкладки свой стек: при переключении вкладок открывается экран, на котором ушли.
// Переходы анимируются через View Transitions API, если браузер его поддерживает.

export const TABS = ['places', 'map', 'favorites', 'settings'];

export class Router {
  /**
   * @param {Array<{path: string, screen: Function, tab?: string, transition?: string}>} routes
   * @param {(match: object, direction: string) => void} onChange отрисовка найденного маршрута
   */
  constructor(routes, onChange) {
    this.routes = routes.map((r) => ({ ...r, regexp: toRegexp(r.path) }));
    this.onChange = onChange;
    this.extras = new Map(); // параметры, переданные при переходе (например, объект места)
    this.tabStacks = Object.fromEntries(TABS.map((t) => [t, `/${t}`]));
    this.current = null;
    this.depth = 0; // сколько переходов вперёд сделано внутри приложения
  }

  start(initialPath) {
    window.addEventListener('hashchange', () => this._resolve());
    if (!location.hash || location.hash === '#/' ) {
      history.replaceState(null, '', `#${initialPath}`);
    }
    this._resolve();
  }

  get path() {
    return decodeURIComponent(location.hash.slice(1)) || '/';
  }

  /** Переход вперёд. extra — данные для следующего экрана. */
  go(path, extra) {
    if (extra !== undefined) this.extras.set(path.split('?')[0], extra);
    this.depth++;
    location.hash = path;
  }

  /** Переход с заменой текущей записи истории. */
  replace(path, extra) {
    if (extra !== undefined) this.extras.set(path.split('?')[0], extra);
    history.replaceState(null, '', `#${path}`);
    this._resolve();
  }

  back() {
    if (this.depth > 0) {
      this.depth--;
      history.back();
      return;
    }
    // открыли по прямой ссылке — идём к родительскому экрану
    const parent = this.path.split('?')[0].split('/').slice(0, -1).join('/');
    this.replace(parent || '/places');
  }

  /** Нажатие на вкладку: повторное нажатие возвращает к корню вкладки. */
  openTab(tab) {
    const target = this.current?.route.tab === tab ? `/${tab}` : this.tabStacks[tab];
    if (target !== this.path) this.go(target);
  }

  /** Перерисовать текущий экран (например, после смены языка). */
  refresh() {
    this._resolve('none');
  }

  _resolve(forced) {
    const [pathname, query = ''] = this.path.split('?');
    const match = this._match(pathname);
    if (!match) {
      this.replace('/places');
      return;
    }
    match.query = Object.fromEntries(new URLSearchParams(query));
    match.extra = this.extras.get(pathname);
    const direction = forced ?? this._direction(match);
    if (match.route.tab) this.tabStacks[match.route.tab] = this.path;
    this.current = match;
    this.onChange(match, direction);
  }

  _match(pathname) {
    for (const route of this.routes) {
      const m = pathname.match(route.regexp);
      if (m) return { route, params: m.groups ?? {}, pathname };
    }
    return null;
  }

  /** Направление анимации: forward / back / tab / none. */
  _direction(next) {
    const prev = this.current;
    if (!prev) return 'none';
    if (prev.route.tab && next.route.tab && prev.route.tab !== next.route.tab) return 'tab';
    const prevDepth = prev.pathname.split('/').length;
    const nextDepth = next.pathname.split('/').length;
    if (nextDepth > prevDepth) return next.route.transition ?? 'forward';
    if (nextDepth < prevDepth) return prev.route.transition === 'search' ? 'search-back' : 'back';
    return 'none';
  }
}

/** '/places/:id' -> /^\/places\/(?<id>[^/]+)$/ */
function toRegexp(path) {
  const pattern = path.replace(/:(\w+)/g, '(?<$1>[^/]+)');
  return new RegExp(`^${pattern}$`);
}

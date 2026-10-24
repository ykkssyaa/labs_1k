// Маленький помощник для создания DOM-элементов.
// h('div', { class: 'x', onClick: fn }, 'текст', child) — аналог вложенных виджетов во Flutter.

export function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [key, value] of Object.entries(props ?? {})) {
    if (value == null || value === false) continue;
    if (key === 'class') el.className = value;
    else if (key === 'style' && typeof value === 'object') setStyle(el, value);
    else if (key === 'dataset') Object.assign(el.dataset, value);
    else if (key.startsWith('on')) el.addEventListener(key.slice(2).toLowerCase(), value);
    else if (key in el && typeof value !== 'string') el[key] = value;
    else el.setAttribute(key, value === true ? '' : value);
  }
  append(el, children);
  return el;
}

function setStyle(el, style) {
  for (const [name, value] of Object.entries(style)) {
    // CSS-переменные (--icon) задаются только через setProperty
    if (name.startsWith('--')) el.style.setProperty(name, value);
    else el.style[name] = value;
  }
}

function append(el, children) {
  for (const child of children.flat(Infinity)) {
    if (child == null || child === false) continue;
    el.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
}

/** Иконка из assets/icons, красится через currentColor (CSS mask). */
export function icon(name, cls = '') {
  const path = name.endsWith('.svg') ? name : `assets/icons/${name}.svg`;
  return h('span', { class: `icon ${cls}`, 'aria-hidden': 'true', style: { '--icon': iconUrl(path) } });
}

/** url() для CSS-переменной: путь делаем абсолютным, иначе он считается от css/app.css. */
export function iconUrl(path) {
  return `url("${new URL(path, document.baseURI).href}")`;
}

/** Заменяет содержимое контейнера. */
export function render(container, ...children) {
  container.replaceChildren();
  append(container, children);
}

export function debounce(fn, ms) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

// Локализация. Строки хранятся в ru.json / en.json в формате ARB:
// ключи сгруппированы по фичам, у каждого ключа "@ключ" с описанием и плейсхолдерами.
// Поддерживаются {name}, {count, plural, =0{..} one{..} few{..} many{..} other{..}},
// форматы плейсхолдеров: числа, валюта (format: currency), даты (type: DateTime).

export const SUPPORTED_LOCALES = ['ru', 'en'];
export const FALLBACK_LOCALE = 'ru';

/** Язык по выбору пользователя ('system' | 'ru' | 'en') и языкам системы. */
export function resolveLocale(choice, systemLanguages = []) {
  if (SUPPORTED_LOCALES.includes(choice)) return choice;
  for (const lang of systemLanguages) {
    const code = lang.toLowerCase().split('-')[0];
    if (SUPPORTED_LOCALES.includes(code)) return code;
  }
  return FALLBACK_LOCALE;
}

export class L10n {
  /** @param {Record<string, object>} messages ARB по языкам */
  constructor(messages, locale = FALLBACK_LOCALE) {
    this.messages = messages;
    this.locale = locale;
  }

  /** Строка по ключу вида 'feature.key'. */
  t(key, params = {}) {
    const found = lookup(this.messages[this.locale], key) ?? lookup(this.messages[FALLBACK_LOCALE], key);
    if (found == null) {
      console.warn('no translation for', key);
      return key;
    }
    return this.format(found.message, params, found.meta?.placeholders ?? {});
  }

  number(value, options) {
    return new Intl.NumberFormat(this.locale, options).format(value);
  }

  currency(value, currency = 'RUB') {
    return new Intl.NumberFormat(this.locale, { style: 'currency', currency }).format(value);
  }

  date(value, options = { dateStyle: 'long' }) {
    return new Intl.DateTimeFormat(this.locale, options).format(new Date(value));
  }

  /** Расстояние: до 1000 м — в метрах, дальше — в километрах. */
  distance(meters) {
    if (meters < 1000) return `${this.number(Math.round(meters))} ${this.t('units.m')}`;
    return `${this.number(meters / 1000, { maximumFractionDigits: 1 })} ${this.t('units.km')}`;
  }

  format(message, params, placeholders) {
    let out = '';
    for (let i = 0; i < message.length; i++) {
      if (message[i] !== '{') {
        out += message[i];
        continue;
      }
      const end = matchingBrace(message, i);
      out += this._formatArgument(message.slice(i + 1, end), params, placeholders);
      i = end;
    }
    return out;
  }

  _formatArgument(body, params, placeholders) {
    const [name, kind] = body.split(',').map((s) => s.trim());
    const value = params[name];
    if (kind === 'plural') {
      const options = parsePluralOptions(body.slice(body.indexOf('plural') + 'plural'.length + 1));
      const exact = options[`=${value}`];
      const text = exact ?? options[new Intl.PluralRules(this.locale).select(value)] ?? options.other ?? '';
      return this.format(text.replaceAll('#', this.number(value)), params, placeholders);
    }
    if (value == null) return '';
    const meta = placeholders[name] ?? {};
    if (meta.type === 'DateTime') return this.date(value);
    if (meta.format === 'currency') return this.currency(value, meta.optionalParameters?.name ?? 'RUB');
    if (typeof value === 'number') return this.number(value);
    return String(value);
  }
}

function lookup(arb, key) {
  if (!arb) return null;
  const path = key.split('.');
  const last = path.pop();
  let node = arb;
  for (const part of path) {
    node = node?.[part];
  }
  if (typeof node?.[last] !== 'string') return null;
  return { message: node[last], meta: node[`@${last}`] };
}

function matchingBrace(s, start) {
  let depth = 0;
  for (let i = start; i < s.length; i++) {
    if (s[i] === '{') depth++;
    if (s[i] === '}' && --depth === 0) return i;
  }
  throw new Error(`unbalanced braces in "${s}"`);
}

/** "=0{нет} one{# место} other{# мест}" -> { '=0': 'нет', one: '# место', other: '# мест' } */
function parsePluralOptions(s) {
  const options = {};
  let i = 0;
  while (i < s.length) {
    const open = s.indexOf('{', i);
    if (open < 0) break;
    const key = s.slice(i, open).trim();
    const close = matchingBrace(s, open);
    options[key] = s.slice(open + 1, close);
    i = close + 1;
  }
  return options;
}

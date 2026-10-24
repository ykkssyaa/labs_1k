// Общие виджеты приложения: карточка места, кнопка избранного, скелетон, ошибки, заглушки.

import { h, icon } from './dom.js';
import { placeTypeIcon } from '../domain/place.js';
import {
  InternalServerFailure,
  NoNetworkFailure,
  ServerNotRespondingFailure,
  TooManyRequestsFailure,
} from '../core/failures.js';

/** Кнопка «Назад». */
export function backButton(app, onClick = () => app.router.back()) {
  return h('button', { class: 'icon-btn back-btn', 'aria-label': app.l10n.t('common.back'), onClick }, icon('arrow'));
}

/** Шапка экрана с заголовком и кнопками. */
export function appBar({ title, leading = null, actions = [] }) {
  return h('header', { class: 'app-bar' }, h('div', { class: 'app-bar__leading' }, leading), h('h1', { class: 'app-bar__title' }, title), h('div', { class: 'app-bar__actions' }, actions));
}

/** Загрузчик (крутилка). */
export function loader(size = 'medium') {
  return h('div', { class: `loader loader--${size}`, role: 'progressbar' });
}

/** Звёзды рейтинга: «★ 4.5 (2)». */
export function ratingBadge(app, place) {
  if (!place.rating) return null;
  return h(
    'span',
    { class: 'rating-badge', title: app.l10n.t('reviews.count', { count: place.reviewCount }) },
    '★ ',
    app.l10n.number(place.rating, { minimumFractionDigits: 1 }),
    h('span', { class: 'rating-badge__count' }, ` (${place.reviewCount})`),
  );
}

/**
 * Кнопка «в избранное». type: 'small' (на карточке) | 'big' (на экране места).
 * onChange(isFavorite) вызывается после переключения.
 */
export function favoritesButton(app, place, { type = 'small', onChange } = {}) {
  const { favoritesRepository: repo, l10n } = app;
  const button = h('button', { class: `fav-btn fav-btn--${type}`, 'data-testid': 'favorites-button' });

  const update = (animate) => {
    const isFavorite = repo.isFavorite(place.id);
    button.setAttribute('aria-pressed', String(isFavorite));
    button.setAttribute('aria-label', l10n.t(isFavorite ? 'placeDetails.inFavorites' : 'placeDetails.addToFavorites'));
    button.replaceChildren(icon(isFavorite ? 'heart_full' : 'heart', animate && isFavorite ? 'pulse' : ''));
    if (type === 'big') button.append(h('span', {}, l10n.t(isFavorite ? 'placeDetails.inFavorites' : 'placeDetails.addToFavorites')));
    onChange?.(isFavorite);
  };

  button.addEventListener('click', async (e) => {
    e.stopPropagation();
    await repo.toggle(place);
    update(true);
  });
  update(false);
  return button;
}

/** Карточка места. */
export function placeCard(app, place, { onPressed, hero = true } = {}) {
  const card = h('article', { class: 'place-card', 'data-testid': 'place-card', dataset: { id: place.id } });
  const image = h(
    'div',
    { class: 'place-card__image', style: hero ? { viewTransitionName: `place-${place.id}` } : {} },
    networkImage(place.images[0], '', place.type),
    h('span', { class: 'place-card__type' }, app.l10n.t(`placeType.${place.type}`).toLowerCase()),
  );
  const content = h(
    'div',
    { class: 'place-card__content' },
    h('h3', { class: 'place-card__name' }, place.name),
    h('p', { class: 'place-card__description' }, place.description),
    ratingBadge(app, place),
  );
  const tap = h('button', { class: 'place-card__tap', 'aria-label': place.name, onClick: () => onPressed?.(place, card) });
  const fav = favoritesButton(app, place, { onChange: (isFav) => card.classList.toggle('place-card--favorite', isFav) });
  card.append(image, content, tap, h('div', { class: 'place-card__fav' }, fav));
  return card;
}

/** Скелетон карточки с анимацией shimmer. */
export function skeletonCard() {
  return h(
    'div',
    { class: 'place-card place-card--skeleton', 'data-testid': 'skeleton-card' },
    h('div', { class: 'skeleton skeleton--image' }),
    h('div', { class: 'place-card__content' }, h('div', { class: 'skeleton skeleton--line' }), h('div', { class: 'skeleton skeleton--line short' })),
  );
}

/** Картинка из сети. Пока грузится или если не загрузилась — заглушка с иконкой категории. */
export function networkImage(url, alt = '', type = 'other') {
  const img = h('img', { class: 'network-image', src: url ?? '', alt, loading: 'lazy', decoding: 'async' });
  img.addEventListener('load', () => img.classList.add('loaded'));
  img.addEventListener('error', () => img.classList.add('failed'));
  return h('div', { class: 'network-image__wrap', dataset: { type } }, icon(placeTypeIcon(type), 'network-image__fallback'), img);
}

/** Иконка категории в цветном круге. */
export function categoryIcon(type, cls = '') {
  return h('span', { class: `category-icon ${cls}` }, icon(placeTypeIcon(type)));
}

/** Текст ошибки по типу Failure. */
export function errorConfig(app, failure) {
  const t = (k) => app.l10n.t(k);
  if (failure instanceof NoNetworkFailure) {
    return { icon: 'error_delete', title: t('errors.noInternetTitle'), description: t('errors.noInternetDescription') };
  }
  if (
    failure instanceof ServerNotRespondingFailure ||
    failure instanceof InternalServerFailure ||
    failure instanceof TooManyRequestsFailure
  ) {
    return { icon: 'error_go', title: t('errors.serverTitle'), description: t('errors.serverDescription') };
  }
  return { icon: 'error_card', title: t('errors.defaultTitle'), description: t('errors.defaultDescription') };
}

/** Заглушка ошибки с кнопкой «Повторить». */
export function errorPlaceholder(app, failure, onRetry) {
  const config = errorConfig(app, failure);
  return h(
    'div',
    { class: 'placeholder placeholder--error', role: 'alert', 'data-testid': 'error-placeholder', dataset: { failure: failure?.constructor?.name } },
    icon(config.icon, 'placeholder__icon'),
    h('h2', {}, config.title),
    h('p', {}, config.description),
    onRetry && h('button', { class: 'text-btn', onClick: onRetry }, icon('refresh'), app.l10n.t('errors.retry')),
  );
}

/** Пустое состояние. */
export function emptyState({ iconName, title, description }) {
  return h('div', { class: 'placeholder' }, icon(iconName, 'placeholder__icon'), h('h2', {}, title), h('p', {}, description));
}

/** Всплывающее сообщение внизу экрана. */
export function showToast(text) {
  const toast = h('div', { class: 'toast', role: 'status' }, text);
  document.body.append(toast);
  setTimeout(() => toast.classList.add('toast--hide'), 2200);
  setTimeout(() => toast.remove(), 2600);
}

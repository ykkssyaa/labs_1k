import { h, icon, render } from '../../uikit/dom.js';
import { backButton, errorPlaceholder, favoritesButton, loader, networkImage, ratingBadge } from '../../uikit/widgets.js';
import { reviewsBlock } from './reviews_block.js';

/** Детальная информация о месте. Место приходит параметром перехода или грузится по id. */
export function placeDetailScreen(app, match) {
  const id = Number(match.params.id);
  const el = h('div', { class: 'screen place-detail' });

  async function load() {
    // данные из перехода показываем сразу, потом обновляем с сервера
    if (match.extra) show(match.extra);
    else render(el, h('div', { class: 'center' }, backButton(app), loader()));

    const result = await app.placesRepository.getPlaceDetails(id);
    if (!result.ok) {
      if (!match.extra) render(el, backButton(app), errorPlaceholder(app, result.error, load));
      return;
    }
    // расстояние приходит только в списке, поэтому берём его из переданного места
    const place = { ...result.data, distance: result.data.distance ?? match.extra?.distance ?? null };
    // перерисовываем, только если данные изменились, чтобы не сбросить галерею и форму отзыва
    if (JSON.stringify(place) !== JSON.stringify(match.extra)) show(place);
  }

  function show(place) {
    render(el, gallery(app, place), content(app, place));
  }

  load();
  return { el };
}

function gallery(app, place) {
  const images = place.images.length ? place.images : [null];
  const slider = h('div', { class: 'gallery__slider' }, images.map((url) => networkImage(url, place.name, place.type)));
  const bars = h('div', { class: 'gallery__indicator' }, images.map(() => h('span')));
  const counter = h('span', { class: 'gallery__count' }, app.l10n.t('placeDetails.photos', { count: place.images.length }));

  const update = () => {
    const i = Math.round(slider.scrollLeft / slider.clientWidth);
    [...bars.children].forEach((b, j) => b.classList.toggle('active', j === i));
  };
  slider.addEventListener('scroll', update);
  requestAnimationFrame(update);

  return h(
    'div',
    { class: 'gallery', style: { viewTransitionName: `place-${place.id}` } },
    slider,
    bars,
    counter,
    h('div', { class: 'gallery__back' }, backButton(app)),
  );
}

function content(app, place) {
  const { l10n } = app;
  const routeUrl = `https://yandex.ru/maps/?rtext=~${place.lat},${place.lon}`;
  return h(
    'div',
    { class: 'place-detail__content' },
    h('h1', { class: 'place-detail__name' }, place.name),
    h(
      'div',
      { class: 'place-detail__meta' },
      h('span', { class: 'place-detail__type' }, l10n.t(`placeType.${place.type}`).toLowerCase()),
      place.distance != null && h('span', {}, l10n.t('placeDetails.distance', { distance: l10n.distance(place.distance) })),
      ratingBadge(app, place),
    ),
    h('p', { class: 'place-detail__description' }, place.description),
    h('a', { class: 'main-btn', href: routeUrl, target: '_blank', rel: 'noopener' }, icon('go'), l10n.t('placeDetails.route')),
    h('hr'),
    h('div', { class: 'place-detail__actions' }, favoritesButton(app, place, { type: 'big' })),
    reviewsBlock(app, place),
  );
}

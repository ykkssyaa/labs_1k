import { h, render } from '../../uikit/dom.js';
import { errorPlaceholder, loader, showToast } from '../../uikit/widgets.js';

const AUTHOR_KEY = 'review_author';

/** Отзывы о месте: список и форма с оценкой 1–5. */
export function reviewsBlock(app, place) {
  const { l10n } = app;
  const title = h('h2', { class: 'reviews__title' }, l10n.t('reviews.title'));
  const list = h('div', { class: 'reviews__list', 'data-testid': 'reviews-list' });

  let rating = 0;
  const stars = h(
    'div',
    { class: 'stars', role: 'radiogroup', 'aria-label': l10n.t('reviews.rating') },
    [1, 2, 3, 4, 5].map((n) =>
      h('button', { type: 'button', class: 'star', role: 'radio', 'aria-label': String(n), onClick: () => setRating(n) }, '★'),
    ),
  );
  const author = h('input', {
    class: 'text-field',
    name: 'author',
    maxLength: 50,
    placeholder: l10n.t('reviews.author'),
    value: app.keyValue.getString(AUTHOR_KEY) ?? '',
    required: true,
  });
  const text = h('textarea', { class: 'text-field', name: 'text', maxLength: 1000, rows: 3, placeholder: l10n.t('reviews.text') });
  const send = h('button', { class: 'main-btn', type: 'submit', disabled: true }, l10n.t('reviews.send'));
  const form = h('form', { class: 'reviews__form', onSubmit: submit, onInput: validate }, stars, author, text, send);

  function setRating(n) {
    rating = n;
    [...stars.children].forEach((s, i) => {
      s.classList.toggle('active', i < n);
      s.setAttribute('aria-checked', String(i + 1 === n));
    });
    validate();
  }

  function validate() {
    send.disabled = rating === 0 || !author.value.trim();
  }

  async function submit(e) {
    e.preventDefault();
    send.disabled = true;
    const result = await app.reviewsRepository.addReview(place.id, { author: author.value.trim(), rating, text: text.value.trim() });
    if (!result.ok) {
      showToast(l10n.t('errors.defaultTitle'));
      validate();
      return;
    }
    app.keyValue.setString(AUTHOR_KEY, author.value.trim());
    text.value = '';
    setRating(0);
    showToast(l10n.t('reviews.sent'));
    load();
  }

  async function load() {
    render(list, loader('small'));
    const result = await app.reviewsRepository.getReviews(place.id);
    if (!result.ok) {
      render(list, errorPlaceholder(app, result.error, load));
      return;
    }
    const reviews = result.data;
    title.textContent = `${l10n.t('reviews.title')} · ${l10n.t('reviews.count', { count: reviews.length })}`;
    if (reviews.length === 0) {
      render(list, h('p', { class: 'reviews__empty' }, l10n.t('reviews.empty')));
      return;
    }
    render(list, reviews.map((r) => reviewItem(app, r)));
  }

  load();
  return h('section', { class: 'reviews' }, title, form, list);
}

function reviewItem(app, review) {
  return h(
    'article',
    { class: 'review' },
    h(
      'div',
      { class: 'review__head' },
      h('b', {}, review.author),
      h('span', { class: 'review__stars', 'aria-label': `${review.rating}/5` }, '★'.repeat(review.rating) + '☆'.repeat(5 - review.rating)),
      h('time', { datetime: review.createdAt }, app.l10n.t('reviews.date', { date: review.createdAt })),
    ),
    review.text && h('p', {}, review.text),
  );
}

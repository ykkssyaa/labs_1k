import { h } from '../../uikit/dom.js';
import { OnboardingStepType, onboardingStepExtension } from './onboarding_steps.js';

/** Онбординг: три страницы с листанием, точки-индикатор, «Пропустить» и «На старт». */
export function onboardingScreen(app, match) {
  const { l10n } = app;
  let current = 0;

  const pages = OnboardingStepType.map((step) => {
    const { icon, title, description } = onboardingStepExtension(step, l10n);
    return h(
      'section',
      { class: 'onboarding__page' },
      h('img', { class: 'onboarding__icon stagger', src: icon, alt: '' }),
      h('h2', { class: 'onboarding__title stagger' }, title),
      h('p', { class: 'onboarding__description stagger' }, description),
    );
  });
  const pager = h('div', { class: 'onboarding__pager', tabIndex: 0 }, pages);
  const dots = h(
    'div',
    { class: 'dots' },
    pages.map((_, i) => h('button', { class: 'dot', 'aria-label': `${i + 1}`, onClick: () => goTo(i) })),
  );
  const skip = h('button', { class: 'text-btn onboarding__skip', onClick: finish }, l10n.t('onboarding.skip'));
  const start = h('button', { class: 'main-btn onboarding__start', onClick: finish }, l10n.t('onboarding.start'));

  function goTo(i) {
    pager.scrollTo({ left: i * pager.clientWidth, behavior: 'smooth' });
  }

  function update() {
    pages.forEach((p, i) => p.classList.toggle('active', i === current));
    [...dots.children].forEach((d, i) => d.classList.toggle('active', i === current));
    const isLast = current === pages.length - 1;
    skip.style.visibility = isLast ? 'hidden' : 'visible';
    start.classList.toggle('visible', isLast);
  }

  function finish() {
    app.onboardingStorage.setDone();
    if (match.query.from === 'settings') app.router.back();
    else app.router.replace('/places');
  }

  pager.addEventListener('scroll', () => {
    const i = Math.round(pager.scrollLeft / pager.clientWidth);
    if (i !== current) {
      current = i;
      update();
    }
  });
  pager.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowRight') goTo(Math.min(current + 1, pages.length - 1));
    if (e.key === 'ArrowLeft') goTo(Math.max(current - 1, 0));
  });

  update();
  const el = h('div', { class: 'screen onboarding' }, h('div', { class: 'onboarding__top' }, skip), pager, dots, start);
  return { el, onShow: () => pager.focus() };
}

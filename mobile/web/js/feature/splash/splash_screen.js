import { h } from '../../uikit/dom.js';

const SPLASH_DURATION = 1200;

/** Сплэш: логотип, пока идёт инициализация; потом онбординг (первый запуск) или список мест. */
export function splashScreen(app) {
  const el = h('div', { class: 'splash', 'data-testid': 'splash' }, h('img', { class: 'splash__logo', src: 'assets/icons/splash_logo.svg', alt: '' }));
  const timer = setTimeout(() => {
    app.router.replace(app.onboardingStorage.isDone() ? '/places' : '/onboarding');
  }, SPLASH_DURATION);
  return { el, dispose: () => clearTimeout(timer) };
}

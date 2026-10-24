// Шаги онбординга и их содержимое (аналог OnboardingStepExtension).

export const OnboardingStepType = ['first', 'second', 'third'];

const icons = {
  first: 'assets/icons/onboarding_page_1.svg',
  second: 'assets/icons/onboarding_page_2.svg',
  third: 'assets/icons/onboarding_page_3.svg',
};

const keys = { first: 'page1', second: 'page2', third: 'page3' };

/** Иконка, заголовок и текст шага. */
export function onboardingStepExtension(step, l10n) {
  if (!OnboardingStepType.includes(step)) throw new Error(`unknown onboarding step: ${step}`);
  return {
    icon: icons[step],
    title: l10n.t(`onboarding.${keys[step]}Title`),
    description: l10n.t(`onboarding.${keys[step]}Description`),
  };
}

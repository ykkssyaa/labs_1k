import { test } from 'node:test';
import assert from 'node:assert/strict';
import { OnboardingStepType, onboardingStepExtension } from '../../web/js/feature/onboarding/onboarding_steps.js';
import { createL10n } from './helpers.js';

const l10n = createL10n('ru');

test('у каждого шага своя иконка, заголовок и описание', () => {
  const steps = OnboardingStepType.map((s) => onboardingStepExtension(s, l10n));
  assert.equal(steps.length, 3);
  assert.equal(new Set(steps.map((s) => s.icon)).size, 3);
  assert.equal(new Set(steps.map((s) => s.title)).size, 3);
  assert.equal(new Set(steps.map((s) => s.description)).size, 3);
});

test('первый шаг', () => {
  const step = onboardingStepExtension('first', l10n);
  assert.equal(step.icon, 'assets/icons/onboarding_page_1.svg');
  assert.equal(step.title, 'Добро пожаловать\nв Путеводитель');
  assert.equal(step.description, 'Ищи новые локации и сохраняй\nсамые любимые.');
});

test('третий шаг на английском', () => {
  const step = onboardingStepExtension('third', createL10n('en'));
  assert.equal(step.icon, 'assets/icons/onboarding_page_3.svg');
  assert.equal(step.title, 'Add places\nyou found yourself');
});

test('неизвестный шаг — ошибка', () => {
  assert.throws(() => onboardingStepExtension('fourth', l10n));
});

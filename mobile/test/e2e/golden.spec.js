// Golden-тесты: сравнение скриншотов карточки места с эталоном (test/e2e/goldens).
// Обновить эталоны: PW_ARGS=--update-snapshots docker compose --profile test run --rm tests
import { test, expect } from '@playwright/test';
import { mockApi, skipOnboarding, stubImages } from './fixtures.js';

test.beforeEach(async ({ page }) => {
  await stubImages(page);
  await skipOnboarding(page);
});

test('карточка места PlaceCardWidget', async ({ page }) => {
  await mockApi(page);
  await page.goto('/#/places');
  const card = page.getByTestId('place-card').first();
  await expect(card.locator('img.loaded')).toHaveCount(1);
  await expect(card).toHaveScreenshot('place_card.png', { animations: 'disabled' });
});

test('карточка места в тёмной теме', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('theme_mode', 'dark'));
  await mockApi(page);
  await page.goto('/#/places');
  const card = page.getByTestId('place-card').first();
  await expect(card.locator('img.loaded')).toHaveCount(1);
  await expect(card).toHaveScreenshot('place_card_dark.png', { animations: 'disabled' });
});

test('скелетон карточки SkeletonPlaceCardWidget', async ({ page }) => {
  await mockApi(page, { delay: Infinity });
  await page.goto('/#/places');
  await expect(page.getByTestId('skeleton-card').first()).toHaveScreenshot('skeleton_place_card.png', { animations: 'disabled' });
});

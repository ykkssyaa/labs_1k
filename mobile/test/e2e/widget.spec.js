// «Виджет»-тесты: отдельные элементы интерфейса на подменённых данных.
import { test, expect } from '@playwright/test';
import { mockApi, skipOnboarding, stubImages } from './fixtures.js';

test.beforeEach(async ({ page }) => {
  await stubImages(page);
  await skipOnboarding(page);
  await mockApi(page);
});

test('кнопка «В избранное» переключает состояние по нажатию', async ({ page }) => {
  await page.goto('/#/places/1');
  const button = page.locator('.place-detail__actions').getByTestId('favorites-button');

  await expect(button).toHaveAttribute('aria-pressed', 'false');
  await expect(button).toHaveText('В Избранное');

  await button.click();
  await expect(button).toHaveAttribute('aria-pressed', 'true');
  await expect(button).toHaveText('В Избранном');

  await button.click();
  await expect(button).toHaveAttribute('aria-pressed', 'false');
});

test('маленькая кнопка избранного на карточке красит карточку', async ({ page }) => {
  await page.goto('/#/places');
  const card = page.getByTestId('place-card').first();
  await card.getByTestId('favorites-button').click();
  await expect(card.getByTestId('favorites-button')).toHaveAttribute('aria-pressed', 'true');
  await expect(card).toHaveClass(/place-card--favorite/);
});

test('нижняя навигация: все вкладки на месте и переключаются', async ({ page }) => {
  await page.goto('/#/places');
  const nav = page.getByTestId('bottom-nav');
  const items = nav.getByRole('button');

  await expect(items).toHaveCount(4);
  for (const label of ['Список мест', 'Карта', 'Избранное', 'Настройки']) {
    await expect(nav.getByRole('button', { name: label })).toBeVisible();
  }
  await expect(nav.getByRole('button', { name: 'Список мест' })).toHaveAttribute('aria-current', 'page');

  await nav.getByRole('button', { name: 'Настройки' }).click();
  await expect(page).toHaveURL(/#\/settings$/);
  await expect(nav.getByRole('button', { name: 'Настройки' })).toHaveAttribute('aria-current', 'page');
});

test('ошибка сервера показывает заглушку с кнопкой «Повторить»', async ({ page }) => {
  await page.route(/\/filtered_places$/, (route) => route.fulfill({ status: 400, json: { error: 'bad' } }));
  await page.goto('/#/places');
  const error = page.getByTestId('error-placeholder');
  await expect(error).toBeVisible();
  await expect(error).toHaveAttribute('data-failure', 'BadRequestFailure');
  await expect(error.getByRole('button', { name: 'Повторить' })).toBeVisible();
});

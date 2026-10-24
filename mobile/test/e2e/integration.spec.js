// Интеграционные тесты: настоящее приложение с настоящим бэкендом (подменяются только фото).
import { test, expect } from '@playwright/test';
import { skipOnboarding, stubImages } from './fixtures.js';

test.beforeEach(async ({ page }) => {
  await stubImages(page);
});

test('список -> детали -> добавление в избранное', async ({ page }) => {
  await skipOnboarding(page);
  // 1) находимся на экране списка мест
  await page.goto('/#/places');
  const card = page.getByTestId('place-card').first();
  await expect(card).toBeVisible();
  const name = await card.locator('.place-card__name').textContent();

  // 2) нажали на карточку места
  await card.locator('.place-card__tap').click();

  // 3) отобразился экран деталей места
  await expect(page).toHaveURL(/#\/places\/\d+$/);
  await expect(page.locator('.place-detail__name')).toHaveText(name);

  // 4) нажали на кнопку добавления в избранное
  const button = page.locator('.place-detail__actions').getByTestId('favorites-button');
  await expect(button).toHaveAttribute('aria-pressed', 'false');
  await button.click();

  // 5) кнопка изменила своё состояние
  await expect(button).toHaveAttribute('aria-pressed', 'true');
  await expect(button).toHaveText('В Избранном');

  // и место появилось на вкладке «Избранное»
  await page.getByTestId('bottom-nav').getByRole('button', { name: 'Избранное' }).click();
  await expect(page.getByTestId('place-card')).toHaveCount(1);
  await expect(page.locator('.place-card__name')).toHaveText(name);
});

test('первый запуск: сплэш -> онбординг -> список мест', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('splash')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Пропустить' })).toBeVisible();
  await page.getByRole('button', { name: 'Пропустить' }).click();
  await expect(page).toHaveURL(/#\/places$/);

  // второй запуск — онбординга уже нет
  await page.goto('/');
  await expect(page).toHaveURL(/#\/places$/, { timeout: 5000 });
});

test('фильтр сохраняется и применяется после перезапуска', async ({ page }) => {
  await skipOnboarding(page);
  await page.goto('/#/places/filter');
  await page.getByRole('button', { name: 'Очистить' }).click();
  // оставляем только парки
  for (const type of ['Храм', 'Памятник', 'Театр', 'Музей', 'Отель', 'Ресторан', 'Кафе', 'Особое место']) {
    await page.locator('.category', { hasText: type }).click();
  }
  const show = page.getByTestId('show-button');
  await expect(show).toHaveText('Показать (12)');
  await show.click();

  await expect(page.locator('.places__counter')).toHaveText('Найдено 12 мест');
  await page.reload();
  await expect(page.locator('.places__counter')).toHaveText('Найдено 12 мест');
  await expect(page.locator('.place-card__type').first()).toHaveText('парк');
});

test('отзыв: оценка и текст сохраняются на сервере', async ({ page }) => {
  await skipOnboarding(page);
  await page.goto('/#/places/3');
  const author = `Тест ${Date.now()}`;
  await page.getByRole('radio', { name: '4' }).click();
  await page.getByPlaceholder('Ваше имя').fill(author);
  await page.getByPlaceholder('Расскажите о месте').fill('Хорошее место');
  await page.getByRole('button', { name: 'Отправить отзыв' }).click();

  const review = page.locator('.review', { hasText: author });
  await expect(review).toBeVisible();
  await expect(review.locator('.review__stars')).toHaveText('★★★★☆');
});

test('смена языка в настройках сохраняется', async ({ page }) => {
  await skipOnboarding(page);
  await page.goto('/#/settings');
  await page.getByTestId('language-select').selectOption('en');
  await expect(page.locator('.app-bar__title')).toHaveText('Settings');
  await page.reload();
  await expect(page.locator('.app-bar__title')).toHaveText('Settings');
  await page.getByTestId('bottom-nav').getByRole('button', { name: 'Places' }).click();
  await expect(page.locator('.large-title h1')).toHaveText('List of\ninteresting places');
});

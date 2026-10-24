// Общие заготовки для e2e-тестов: подмена картинок и API, пропуск онбординга.

// серая картинка 1x1 вместо фото из интернета, чтобы скриншоты не зависели от сети
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mN8+P9/PQAJYwPMw1xLwQAAAABJRU5ErkJggg==',
  'base64',
);

export const testPlace = {
  id: 1,
  name: 'Государственный Эрмитаж',
  lat: 59.9452,
  lng: 30.3245,
  placeType: 'museum',
  description: 'Один из крупнейших и самых известных художественных музеев мира, расположенный в Санкт-Петербурге.',
  urls: ['https://picsum.photos/1000/600?random=1', 'https://picsum.photos/1000/600?random=2'],
  rating: 4.5,
  reviewCount: 2,
};

export async function stubImages(page) {
  await page.route(/picsum\.photos|tile\.openstreetmap\.org/, (route) => route.fulfill({ body: PNG, contentType: 'image/png' }));
}

export async function skipOnboarding(page) {
  await page.addInitScript(() => localStorage.setItem('onboarding_done', '1'));
}

/** Подмена бэкенда фиксированными данными. delay — «вечная» загрузка, если Infinity. */
export async function mockApi(page, { places = [testPlace], delay = 0 } = {}) {
  const reply = async (route, body) => {
    if (delay === Infinity) return; // запрос не завершается
    if (delay) await new Promise((r) => setTimeout(r, delay));
    await route.fulfill({ json: body });
  };
  await page.route(/\/place$/, (route) => reply(route, places));
  await page.route(/\/filtered_places$/, (route) => reply(route, places));
  await page.route(/\/place\/\d+$/, (route) => {
    const id = Number(route.request().url().split('/').pop());
    return reply(route, places.find((p) => p.id === id));
  });
  await page.route(/\/place\/\d+\/reviews$/, (route) => reply(route, []));
}

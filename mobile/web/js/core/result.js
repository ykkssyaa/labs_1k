import { unwrapError } from './failures.js';

// Результат запроса: { ok: true, data } или { ok: false, error }.
export const ok = (data) => ({ ok: true, data });
export const failed = (error) => ({ ok: false, error });

/** Базовый метод репозиториев: выполняет запрос и заворачивает ошибку в Failure. */
export async function makeApiCall(call) {
  try {
    return ok(await call());
  } catch (e) {
    return failed(unwrapError(e));
  }
}

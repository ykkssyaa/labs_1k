// Ошибки приложения. Сетевые ошибки получаются из HTTP-кодов в unwrapError().

export class Failure extends Error {}

export class NetworkFailure extends Failure {}
export class NoNetworkFailure extends NetworkFailure {}
export class ServerNotRespondingFailure extends NetworkFailure {}
export class BadRequestFailure extends NetworkFailure {}
export class UnauthorizedFailure extends NetworkFailure {}
export class TooManyRequestsFailure extends NetworkFailure {}
export class InternalServerFailure extends NetworkFailure {}
export class UnknownNetworkFailure extends NetworkFailure {}
export class UnknownFailure extends Failure {}

/** Ошибка HTTP-ответа, её выбрасывает ApiClient. */
export class HttpError extends Error {
  constructor(status, body) {
    super(`HTTP ${status}`);
    this.status = status;
    this.body = body;
  }
}

/** Ошибка таймаута запроса. */
export class TimeoutError extends Error {}

/** Переводит исключение запроса в Failure. */
export function unwrapError(error) {
  if (error instanceof Failure) return error;
  if (error instanceof TimeoutError) return new ServerNotRespondingFailure(error.message);
  if (error instanceof HttpError) {
    const message = error.body?.error ?? error.message;
    const s = error.status;
    if (s === 401 || s === 403) return new UnauthorizedFailure(message);
    if (s === 429) return new TooManyRequestsFailure(message);
    if (s >= 400 && s < 500) return new BadRequestFailure(message);
    if (s === 502 || s === 504) return new ServerNotRespondingFailure(message);
    if (s >= 500) return new InternalServerFailure(message);
    return new UnknownNetworkFailure(message);
  }
  // fetch бросает TypeError, когда сети нет или сервер недоступен
  if (error instanceof TypeError) return new NoNetworkFailure(error.message);
  return new UnknownFailure(String(error?.message ?? error));
}

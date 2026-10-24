/** Значение с подписчиками (аналог ValueNotifier / BehaviorSubject). */
export class ValueNotifier {
  constructor(value) {
    this._value = value;
    this._listeners = new Set();
  }

  get value() {
    return this._value;
  }

  set value(v) {
    this._value = v;
    this._listeners.forEach((fn) => fn(v));
  }

  /** Подписка, возвращает функцию отписки. */
  listen(fn) {
    this._listeners.add(fn);
    return () => this._listeners.delete(fn);
  }
}

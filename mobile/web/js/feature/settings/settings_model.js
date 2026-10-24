/** Модель экрана настроек: тема и язык. */
export class SettingsModel {
  constructor({ themeModeRepository, localeRepository }) {
    this.themeModeRepository = themeModeRepository;
    this.localeRepository = localeRepository;
  }

  get currentThemeMode() {
    return this.themeModeRepository.themeModeStream.value;
  }

  setThemeMode(mode) {
    if (mode !== 'light' && mode !== 'dark') throw new Error(`unknown theme mode: ${mode}`);
    this.themeModeRepository.setThemeMode(mode);
  }

  get currentLocale() {
    return this.localeRepository.localeStream.value;
  }

  setLocale(locale) {
    this.localeRepository.setLocale(locale);
  }
}

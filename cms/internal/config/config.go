// Package config читает настройки из окружения и файла .env.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Addr          string
	DBPath        string
	UploadsDir    string
	Theme         string
	BaseURL       string
	AdminEmail    string
	AdminPassword string
	Seed          bool
	// Каталоги с конфигурацией контента; в тестах подменяются.
	ContentTypesDir string
	RolesFile       string
	ThemesDir       string
}

// Load читает .env (если есть) и окружение. Переменные окружения важнее .env.
func Load(envFile string) (Config, error) {
	fileVars, err := readEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}
	get := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		if v, ok := fileVars[key]; ok {
			return v
		}
		return def
	}
	return Config{
		Addr:            get("VESTNIK_ADDR", ":8080"),
		DBPath:          get("VESTNIK_DB", "data/vestnik.db"),
		UploadsDir:      get("VESTNIK_UPLOADS", "data/uploads"),
		Theme:           get("VESTNIK_THEME", "classic"),
		BaseURL:         strings.TrimRight(get("VESTNIK_BASE_URL", "http://localhost:8080"), "/"),
		AdminEmail:      get("VESTNIK_ADMIN_EMAIL", ""),
		AdminPassword:   get("VESTNIK_ADMIN_PASSWORD", ""),
		Seed:            get("VESTNIK_SEED", "0") == "1",
		ContentTypesDir: get("VESTNIK_CONTENT_TYPES", "content_types"),
		RolesFile:       get("VESTNIK_ROLES", "config/roles.json"),
		ThemesDir:       get("VESTNIK_THEMES", "themes"),
	}, nil
}

// readEnvFile разбирает простой формат KEY=VALUE, строки с # — комментарии.
func readEnvFile(path string) (map[string]string, error) {
	vars := map[string]string{}
	if path == "" {
		return vars, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return vars, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: ожидается KEY=VALUE", path, n)
		}
		vars[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return vars, sc.Err()
}

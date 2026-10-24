package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFileAndOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("# comment\nVESTNIK_ADDR=:9000\nVESTNIK_THEME=\"newspaper\"\nVESTNIK_BASE_URL=http://x/\n"), 0o600)
	t.Setenv("VESTNIK_ADDR", ":7000")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":7000" {
		t.Errorf("окружение должно перекрывать .env: %q", cfg.Addr)
	}
	if cfg.Theme != "newspaper" {
		t.Errorf("кавычки не сняты: %q", cfg.Theme)
	}
	if cfg.BaseURL != "http://x" {
		t.Errorf("хвостовой слэш не убран: %q", cfg.BaseURL)
	}
}

func TestLoadRejectsBrokenLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("JUSTKEY\n"), 0o600)
	if _, err := Load(path); err == nil {
		t.Fatal("ожидалась ошибка разбора")
	}
}

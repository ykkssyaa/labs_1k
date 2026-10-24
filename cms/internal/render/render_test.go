package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeThemes создаёт дерево тем: base ← child (переопределяет header) ← grandchild (новая страница).
func writeThemes(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

var themeTree = map[string]string{
	"base/theme.json":            `{"label": "Base", "public": true}`,
	"base/layouts/base.html":     `{{define "base"}}[{{template "header" .}}|{{block "content" .}}{{end}}|{{template "footer" .}}]{{end}}`,
	"base/partials/header.html":  `{{define "header"}}base-header{{end}}`,
	"base/partials/footer.html":  `{{define "footer"}}base-footer{{end}}`,
	"base/pages/home.html":       `{{define "content"}}home:{{.}}{{end}}`,
	"child/theme.json":           `{"label": "Child", "extends": "base", "public": true}`,
	"child/partials/header.html": `{{define "header"}}child-header{{end}}`,
	"grand/theme.json":           `{"label": "Grand", "extends": "child"}`,
	"grand/pages/extra.html":     `{{define "content"}}extra{{end}}`,
}

func TestThemeInheritance(t *testing.T) {
	e, err := New(writeThemes(t, themeTree), nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ theme, page, want string }{
		{"base", "home", "[base-header|home:<b>|base-footer]"},
		{"child", "home", "[child-header|home:<b>|base-footer]"}, // header переопределён, остальное унаследовано
		{"grand", "home", "[child-header|home:<b>|base-footer]"}, // наследование через два уровня
		{"grand", "extra", "[child-header|extra|base-footer]"},
	}
	for _, c := range cases {
		var sb strings.Builder
		if err := e.Render(&sb, c.theme, c.page, "<b>"); err != nil {
			t.Fatalf("%s/%s: %v", c.theme, c.page, err)
		}
		want := strings.ReplaceAll(c.want, "<b>", "&lt;b&gt;") // данные экранируются
		if sb.String() != want {
			t.Errorf("%s/%s = %q, ожидали %q", c.theme, c.page, sb.String(), want)
		}
	}
	if !e.Has("grand", "home") || e.Has("base", "extra") {
		t.Error("Has неверно учитывает цепочку тем")
	}
	if e.HasTheme("grand") {
		t.Error("тема без public не должна предлагаться для сайта")
	}
	if err := e.Render(&strings.Builder{}, "base", "missing", nil); err == nil {
		t.Error("ожидалась ошибка для отсутствующей страницы")
	}
}

func TestThemeCycleRejected(t *testing.T) {
	_, err := New(writeThemes(t, map[string]string{
		"a/theme.json": `{"label": "A", "extends": "b"}`,
		"b/theme.json": `{"label": "B", "extends": "a"}`,
	}), nil)
	if err == nil || !strings.Contains(err.Error(), "цикл") {
		t.Fatalf("цикл наследования не обнаружен: %v", err)
	}
}

func TestProjectHasTwoPublicThemes(t *testing.T) {
	e, err := New("../../themes", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Themes()) < 2 {
		t.Fatalf("по заданию нужно не меньше двух тем сайта, найдено %d", len(e.Themes()))
	}
	for _, th := range e.Themes() {
		if _, err := e.chain(th.Name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPluralAndTruncate(t *testing.T) {
	for n, want := range map[int]string{1: "материал", 3: "материала", 5: "материалов", 11: "материалов", 21: "материал", 104: "материала"} {
		if got := plural(n, "материал", "материала", "материалов"); got != want {
			t.Errorf("plural(%d) = %s", n, got)
		}
	}
	if got := truncate(5, "Привет, мир"); got != "Приве…" {
		t.Errorf("truncate: %q", got)
	}
	if got := PlainText("## Заголовок\n\n**жирный** [ссылка](https://x.ru)", 100); got != "Заголовок жирный ссылка" {
		t.Errorf("PlainText: %q", got)
	}
}

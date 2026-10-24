package render

import (
	"regexp"
	"strings"
	"testing"
)

func TestMarkdownBlocks(t *testing.T) {
	cases := map[string]string{
		"Абзац один\nстрока два\n\nАбзац два": "<p>Абзац один<br>строка два</p>\n<p>Абзац два</p>\n",
		"## Подзаголовок":                     "<h3>Подзаголовок</h3>\n",
		"# Раздел":                            "<h2>Раздел</h2>\n",
		"- один\n- два":                       "<ul><li>один</li><li>два</li></ul>\n",
		"1. первый\n2. второй":                "<ol><li>первый</li><li>второй</li></ol>\n",
		"> цитата\n> продолжение":             "<blockquote><p>цитата<br>продолжение</p></blockquote>\n",
		"**жирный** и *курсив*":               "<p><strong>жирный</strong> и <em>курсив</em></p>\n",
		"код `a**b**c` тут":                   "<p>код <code>a**b**c</code> тут</p>\n",
		"[сайт](https://spb.ru/?a=1&b=2)":     `<p><a href="https://spb.ru/?a=1&amp;b=2" rel="nofollow noopener">сайт</a></p>` + "\n",
		"[раздел](/section/gorod)":            `<p><a href="/section/gorod">раздел</a></p>` + "\n",
		"":                                    "",
	}
	for in, want := range cases {
		if got := string(Markdown(in)); got != want {
			t.Errorf("Markdown(%q)\n got: %q\nwant: %q", in, got, want)
		}
	}
}

// Тест на внедрение разметки: ничего исполняемого не должно пройти.
func TestMarkdownEscapesInjection(t *testing.T) {
	attacks := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror=alert(1)>`,
		`[клик](javascript:alert(1))`,
		`[клик](JaVaScRiPt:alert(1))`,
		`[клик](  javascript:alert(1))`,
		`[клик](data:text/html;base64,PHNjcmlwdD4=)`,
		`[клик](//evil.example/x)`,
		`[x"><script>alert(1)</script>](https://ok.ru)`,
		`[ok](https://ok.ru/"onmouseover="alert(1))`,
		"**<svg onload=alert(1)>**",
		"`<script>`",
		"> <iframe src=//evil>",
		"- <a href=javascript:alert(1)>x</a>",
	}
	for _, a := range attacks {
		if bad := unsafeMarkup(string(Markdown(a))); bad != "" {
			t.Errorf("вход %q: небезопасная разметка %q", a, bad)
		}
	}
}

var (
	tagRe       = regexp.MustCompile(`<(/?)([a-zA-Z0-9]+)([^>]*)>`)
	allowedTags = map[string]bool{"p": true, "br": true, "strong": true, "em": true, "code": true, "a": true,
		"h2": true, "h3": true, "h4": true, "ul": true, "ol": true, "li": true, "blockquote": true}
	safeHrefRe = regexp.MustCompile(`^ href="(https?://[^"]*|/[^/"][^"]*|#[^"]*|mailto:[^"]*)"( rel="nofollow noopener")?$`)
)

// unsafeMarkup ищет в выводе теги не из белого списка и атрибуты, кроме безопасного href.
// Простой поиск подстрок вроде "onerror=" здесь не годится: экранированный текст
// «&lt;img onerror=…&gt;» безопасен, хотя подстроку содержит.
func unsafeMarkup(out string) string {
	for _, m := range tagRe.FindAllStringSubmatch(out, -1) {
		closing, name, attrs := m[1], strings.ToLower(m[2]), m[3]
		if !allowedTags[name] {
			return m[0]
		}
		if closing != "" || attrs == "" {
			continue
		}
		if name != "a" || !safeHrefRe.MatchString(attrs) {
			return m[0]
		}
	}
	return ""
}

func TestUnsafeMarkupDetector(t *testing.T) {
	for _, bad := range []string{`<script>`, `<a href="javascript:x">`, `<a href="//evil">`, `<p onclick="x">`, `<img src=x>`} {
		if unsafeMarkup(bad) == "" {
			t.Errorf("детектор пропустил %q", bad)
		}
	}
	if s := unsafeMarkup(`<p>&lt;img onerror=x&gt; <a href="/x">y</a></p>`); s != "" {
		t.Errorf("ложное срабатывание: %q", s)
	}
}

func TestMarkdownStripsNUL(t *testing.T) {
	out := string(Markdown("a\x000\x00b `c`"))
	if strings.Contains(out, "\x00") || !strings.Contains(out, "<code>c</code>") {
		t.Fatalf("NUL обработан неверно: %q", out)
	}
}

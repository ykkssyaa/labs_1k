package render

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

// Markdown превращает текст материала в HTML. Это единственное место, где
// пользовательский текст становится template.HTML, поэтому порядок жёсткий:
// сначала экранируем всё, потом добавляем только разрешённую разметку.
func Markdown(src string) template.HTML {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\x00", "") // \x00 занят под временные метки кода в inline
	var out strings.Builder
	for _, block := range splitBlocks(src) {
		lines := strings.Split(block, "\n")
		first := lines[0]
		switch {
		case strings.HasPrefix(first, "### "):
			writeTag(&out, "h4", inline(strings.TrimPrefix(first, "### ")))
		case strings.HasPrefix(first, "## "):
			writeTag(&out, "h3", inline(strings.TrimPrefix(first, "## ")))
		case strings.HasPrefix(first, "# "):
			writeTag(&out, "h2", inline(strings.TrimPrefix(first, "# ")))
		case allHavePrefix(lines, "> "), allHavePrefix(lines, ">"):
			var inner []string
			for _, l := range lines {
				inner = append(inner, strings.TrimSpace(strings.TrimPrefix(l, ">")))
			}
			writeTag(&out, "blockquote", "<p>"+joinLines(inner)+"</p>")
		case allMatch(lines, bulletRe):
			writeList(&out, "ul", lines, bulletRe)
		case allMatch(lines, numberRe):
			writeList(&out, "ol", lines, numberRe)
		default:
			writeTag(&out, "p", joinLines(lines))
		}
		out.WriteByte('\n')
	}
	return template.HTML(out.String())
}

var (
	bulletRe = regexp.MustCompile(`^\s*[-*]\s+`)
	numberRe = regexp.MustCompile(`^\s*\d+[.)]\s+`)
)

func splitBlocks(src string) []string {
	var blocks []string
	for _, b := range regexp.MustCompile(`\n\s*\n`).Split(src, -1) {
		if b = strings.Trim(b, "\n "); b != "" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func allHavePrefix(lines []string, p string) bool {
	for _, l := range lines {
		if !strings.HasPrefix(l, p) {
			return false
		}
	}
	return true
}

func allMatch(lines []string, re *regexp.Regexp) bool {
	for _, l := range lines {
		if !re.MatchString(l) {
			return false
		}
	}
	return true
}

func writeTag(b *strings.Builder, tag, inner string) {
	b.WriteString("<" + tag + ">" + inner + "</" + tag + ">")
}

func writeList(b *strings.Builder, tag string, lines []string, re *regexp.Regexp) {
	b.WriteString("<" + tag + ">")
	for _, l := range lines {
		writeTag(b, "li", inline(re.ReplaceAllString(l, "")))
	}
	b.WriteString("</" + tag + ">")
}

func joinLines(lines []string) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = inline(strings.TrimSpace(l))
	}
	return strings.Join(parts, "<br>")
}

var (
	codeRe   = regexp.MustCompile("`([^`]+)`")
	linkRe   = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	boldRe   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*]+)\*`)
)

// inline экранирует строку и добавляет строчную разметку.
func inline(s string) string {
	s = html.EscapeString(s)
	// Код вынимаем первым, чтобы звёздочки и скобки внутри него не превратились в разметку.
	var codes []string
	s = codeRe.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, "<code>"+codeRe.FindStringSubmatch(m)[1]+"</code>")
		return "\x00" + string(rune('0'+len(codes)-1)) + "\x00"
	})
	s = linkRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := linkRe.FindStringSubmatch(m)
		text, href := parts[1], parts[2]
		safe, external := safeURL(href)
		if safe == "" {
			return text
		}
		rel := ""
		if external {
			rel = ` rel="nofollow noopener"`
		}
		return `<a href="` + safe + `"` + rel + `>` + text + `</a>`
	})
	s = boldRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = italicRe.ReplaceAllString(s, "<em>$1</em>")
	for i, c := range codes {
		s = strings.Replace(s, "\x00"+string(rune('0'+i))+"\x00", c, 1)
	}
	return s
}

// safeURL проверяет адрес ссылки. href приходит уже экранированным; схему
// проверяем по раскодированному значению, а в атрибут кладём экранированное.
func safeURL(escaped string) (string, bool) {
	raw := html.UnescapeString(escaped)
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "#") {
		return escaped, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return escaped, true
	case "mailto":
		return escaped, false
	}
	return "", false
}

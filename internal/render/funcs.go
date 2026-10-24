package render

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"
)

var months = []string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// Moscow — часовой пояс издания для показа дат.
var Moscow = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}()

var builtinFuncs = template.FuncMap{
	// date: «24 октября 2026»
	"date": func(t any) string {
		tm, ok := asTime(t)
		if !ok {
			return ""
		}
		tm = tm.In(Moscow)
		return fmt.Sprintf("%d %s %d", tm.Day(), months[tm.Month()-1], tm.Year())
	},
	// datetime: «24.10.2026 19:30»
	"datetime": func(t any) string {
		tm, ok := asTime(t)
		if !ok {
			return ""
		}
		return tm.In(Moscow).Format("02.01.2006 15:04")
	},
	"isodate": func(t any) string {
		tm, ok := asTime(t)
		if !ok {
			return ""
		}
		return tm.UTC().Format(time.RFC3339)
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	// seq 1 5 -> [1 2 3 4 5], для пагинации
	"seq": func(from, to int) []int {
		var out []int
		for i := from; i <= to; i++ {
			out = append(out, i)
		}
		return out
	},
	// dict "a" 1 "b" 2 — передать несколько значений в partial
	"dict": func(kv ...any) (map[string]any, error) {
		if len(kv)%2 != 0 {
			return nil, fmt.Errorf("dict: нечётное число аргументов")
		}
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			k, ok := kv[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: ключ %v не строка", kv[i])
			}
			m[k] = kv[i+1]
		}
		return m, nil
	},
	"join":     strings.Join,
	"truncate": truncate,
	// query собирает строку запроса, сохраняя фильтры при переходе по страницам
	"query": func(base url.Values, kv ...any) template.URL {
		q := url.Values{}
		for k, v := range base {
			q[k] = append([]string(nil), v...)
		}
		for i := 0; i+1 < len(kv); i += 2 {
			k := fmt.Sprint(kv[i])
			if v := fmt.Sprint(kv[i+1]); v != "" && v != "0" {
				q.Set(k, v)
			} else {
				q.Del(k)
			}
		}
		// Результат — только строка запроса из закодированных значений, безопасна как URL.
		return template.URL("?" + q.Encode())
	},
	"plural":   plural,
	"duration": duration,
}

// duration: «2 ч 15 мин», «40 мин», «—» для нуля.
func duration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h >= 48:
		return fmt.Sprintf("%d дн %d ч", h/24, h%24)
	case h > 0:
		return fmt.Sprintf("%d ч %d мин", h, m)
	}
	return fmt.Sprintf("%d мин", max(m, 1))
}

func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, !t.IsZero()
	case *time.Time:
		if t == nil || t.IsZero() {
			return time.Time{}, false
		}
		return *t, true
	}
	return time.Time{}, false
}

func truncate(n int, s string) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// plural выбирает форму слова: plural 5 "материал" "материала" "материалов".
func plural(n int, one, few, many string) string {
	n10, n100 := n%10, n%100
	switch {
	case n10 == 1 && n100 != 11:
		return one
	case n10 >= 2 && n10 <= 4 && (n100 < 10 || n100 >= 20):
		return few
	}
	return many
}

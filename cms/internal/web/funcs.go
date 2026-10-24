// Package web — общее для панели, сайта и API: функции шаблонов, знающие о контент-модели.
package web

import (
	"context"
	"fmt"
	"html/template"
	"strconv"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/content"
	"github.com/ykkssyaa/vestnik-cms/internal/render"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

// MediaLookup находит медиафайл по id (nil — нет такого).
type MediaLookup func(ctx context.Context, id int64) *store.Media

// MediaID достаёт id медиафайла из значения поля data.
func MediaID(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

// MediaURL — адрес варианта изображения ("" — оригинал).
func MediaURL(m *store.Media, variant string) string {
	if m == nil {
		return ""
	}
	if v, ok := m.Variants[variant]; ok {
		return "/uploads/" + v.Path
	}
	return "/uploads/" + m.Path
}

// Funcs — функции шаблонов, которым нужен реестр типов и медиатека.
func Funcs(types *content.Registry, lookup MediaLookup) template.FuncMap {
	return template.FuncMap{
		"statusLabel": func(s string) string {
			if l, ok := content.StatusLabels[s]; ok {
				return l
			}
			return s
		},
		"typeLabel": func(name string) string {
			if t, ok := types.Get(name); ok {
				return t.Label
			}
			return name
		},
		"typeOf": func(name string) *content.Type {
			t, _ := types.Get(name)
			return t
		},
		// entryPath — человекочитаемый адрес материала на сайте.
		"entryPath": func(e *content.Entry) string {
			if t, ok := types.Get(e.Type); ok {
				return t.Path(e.Slug)
			}
			return "/"
		},
		// fieldText — значение поля data для вывода текстом; отсутствующее — пустая строка.
		"fieldText": func(data map[string]any, name string) string {
			v, ok := data[name]
			if !ok || v == nil {
				return ""
			}
			if b, ok := v.(bool); ok {
				if b {
					return "да"
				}
				return "нет"
			}
			return fmt.Sprint(v)
		},
		"markdown": func(v any) template.HTML {
			s, _ := v.(string)
			return render.Markdown(s)
		},
		// media — медиафайл по значению поля типа media.
		"media": func(v any) *store.Media {
			id := MediaID(v)
			if id == 0 || lookup == nil {
				return nil
			}
			return lookup(context.Background(), id)
		},
		"mediaURL": MediaURL,
		// cover — первое поле типа media у материала (для карточек и og:image).
		"cover": func(e *content.Entry) *store.Media {
			t, ok := types.Get(e.Type)
			if !ok || lookup == nil {
				return nil
			}
			for _, f := range t.Fields {
				if f.Type == content.FieldMedia {
					if id := MediaID(e.Data[f.Name]); id != 0 {
						return lookup(context.Background(), id)
					}
				}
			}
			return nil
		},
		// summary — описание материала: SEO-описание или первое текстовое поле схемы.
		"summary": func(e *content.Entry, n int) string { return Summary(types, e, n) },
		"fmtDateTime": func(v any) string {
			s, _ := v.(string)
			if t, err := parseLocal(s); err == nil {
				return t
			}
			return s
		},
	}
}

var monthsGen = []string{"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// parseLocal форматирует значения полей date/datetime из data («24 октября 2026, 19:30»).
func parseLocal(s string) (string, error) {
	if t, err := time.Parse(content.DateTimeLayout, s); err == nil {
		return fmt.Sprintf("%d %s %d, %s", t.Day(), monthsGen[t.Month()-1], t.Year(), t.Format("15:04")), nil
	}
	t, err := time.Parse(content.DateLayout, s)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %s %d", t.Day(), monthsGen[t.Month()-1], t.Year()), nil
}

// Summary — описание материала для карточек и meta description.
func Summary(types *content.Registry, e *content.Entry, n int) string {
	if e.SEODescription != "" {
		return e.SEODescription
	}
	if t, ok := types.Get(e.Type); ok {
		for _, f := range t.Fields {
			if f.Type == content.FieldText || f.Type == content.FieldMarkdown {
				if s, ok := e.Data[f.Name].(string); ok && s != "" {
					return render.PlainText(s, n)
				}
			}
		}
	}
	return ""
}

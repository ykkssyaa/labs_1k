// Package web — общее для панели, сайта и API: функции шаблонов, знающие о контент-модели.
package web

import (
	"fmt"
	"html/template"

	"github.com/ykkssyaa/vestnik-cms/internal/content"
)

// Funcs — функции шаблонов, которым нужен реестр типов.
func Funcs(types *content.Registry) template.FuncMap {
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
	}
}

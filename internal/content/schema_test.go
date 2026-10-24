package content

import (
	"errors"
	"strings"
	"testing"
)

const testSchema = `{
  "name": "review", "version": 1, "label": "Рецензия", "url_prefix": "reviews",
  "fields": [
    {"name": "subject", "label": "Что", "type": "string", "required": true, "max_length": 10},
    {"name": "kind", "label": "Вид", "type": "select", "required": true, "options": ["книга", "фильм"]},
    {"name": "rating", "label": "Оценка", "type": "int", "required": true, "min": 1, "max": 10},
    {"name": "code", "label": "Код", "type": "string", "pattern": "[A-Z]{3}"},
    {"name": "site", "label": "Сайт", "type": "url"},
    {"name": "hot", "label": "Срочно", "type": "bool"},
    {"name": "day", "label": "День", "type": "date"},
    {"name": "starts", "label": "Начало", "type": "datetime"},
    {"name": "ends", "label": "Конец", "type": "datetime", "after": "starts"},
    {"name": "cover", "label": "Обложка", "type": "media"},
    {"name": "body", "label": "Текст", "type": "markdown", "min_length": 3},
    {"name": "order", "label": "Порядок", "type": "int", "default": 50}
  ]
}`

func mustType(t *testing.T, raw string) *Type {
	t.Helper()
	typ, err := ParseType([]byte(raw))
	if err != nil {
		t.Fatalf("ParseType: %v", err)
	}
	return typ
}

func validBase() map[string]any {
	return map[string]any{"subject": "Дюна", "kind": "фильм", "rating": "8"}
}

func TestValidateFieldRules(t *testing.T) {
	typ := mustType(t, testSchema)
	cases := []struct {
		name    string
		field   string
		value   any
		wantErr bool
		want    any
	}{
		{"string trimmed", "subject", "  Дюна  ", false, "Дюна"},
		{"string max_length in runes", "subject", "Десятьбукв", false, "Десятьбукв"},
		{"string too long", "subject", "Одиннадцать", true, nil},
		{"select ok", "kind", "книга", false, "книга"},
		{"select unknown option", "kind", "сериал", true, nil},
		{"int from form string", "rating", "7", false, int64(7)},
		{"int from json number", "rating", float64(9), false, int64(9)},
		{"int fractional rejected", "rating", 7.5, true, nil},
		{"int below min", "rating", "0", true, nil},
		{"int above max", "rating", 11, true, nil},
		{"int not a number", "rating", "семь", true, nil},
		{"pattern ok", "code", "ABC", false, "ABC"},
		{"pattern is anchored", "code", "xABCx", true, nil},
		{"url ok", "site", "https://example.org/a", false, "https://example.org/a"},
		{"url javascript rejected", "site", "javascript:alert(1)", true, nil},
		{"url without scheme rejected", "site", "example.org", true, nil},
		{"bool checkbox on", "hot", "on", false, true},
		{"bool json", "hot", true, false, true},
		{"bool garbage", "hot", "может быть", true, nil},
		{"date ok", "day", "2026-10-24", false, "2026-10-24"},
		{"date wrong format", "day", "24.10.2026", true, nil},
		{"datetime from html input", "starts", "2026-10-24T19:30", false, "2026-10-24T19:30"},
		{"datetime from rfc3339", "starts", "2026-10-24T19:30:00Z", false, "2026-10-24T19:30"},
		{"media id", "cover", "12", false, int64(12)},
		{"media zero rejected", "cover", 0, true, nil},
		{"markdown min_length", "body", "аб", true, nil},
		{"markdown normalizes newlines", "body", "a\r\nb\n\n", false, "a\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := validBase()
			data[c.field] = c.value
			clean, errs := typ.Validate(data)
			_, hasErr := errs[c.field]
			if hasErr != c.wantErr {
				t.Fatalf("ошибка=%v, ожидали %v (errs=%v)", hasErr, c.wantErr, errs)
			}
			if !c.wantErr && clean[c.field] != c.want {
				t.Fatalf("значение %#v, ожидали %#v", clean[c.field], c.want)
			}
		})
	}
}

func TestValidateRequiredDefaultsAndUnknown(t *testing.T) {
	typ := mustType(t, testSchema)
	clean, errs := typ.Validate(map[string]any{"subject": "  ", "evil": "<script>", "rating": "5", "kind": "книга"})
	if errs["subject"] != "обязательное поле" {
		t.Errorf("пробелы должны считаться пустым обязательным полем: %v", errs)
	}
	if _, ok := clean["evil"]; ok {
		t.Error("неизвестное поле не отброшено")
	}
	if clean["order"] != int64(50) {
		t.Errorf("default не применён: %#v", clean["order"])
	}
	if clean["hot"] != false {
		t.Errorf("отсутствующий bool должен стать false: %#v", clean["hot"])
	}
	if _, ok := clean["site"]; ok {
		t.Error("пустое необязательное поле не должно сохраняться")
	}
}

func TestValidateAfterRule(t *testing.T) {
	typ := mustType(t, testSchema)
	data := validBase()
	data["starts"] = "2026-10-24T19:00"
	data["ends"] = "2026-10-24T18:00"
	if _, errs := typ.Validate(data); errs["ends"] == "" {
		t.Fatal("окончание раньше начала должно быть ошибкой")
	}
	data["ends"] = "2026-10-24T21:00"
	if _, errs := typ.Validate(data); errs != nil {
		t.Fatalf("корректный интервал отклонён: %v", errs)
	}
}

func TestParseTypeRejectsBrokenSchemas(t *testing.T) {
	cases := map[string]string{
		"unknown field type": `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"color"}]}`,
		"duplicate field":    `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"string"},{"name":"a","type":"int"}]}`,
		"select w/o options": `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"select"}]}`,
		"after missing":      `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"date","after":"b"}]}`,
		"after non-date":     `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"string"},{"name":"b","type":"date","after":"a"}]}`,
		"typo in rule":       `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"string","requird":true}]}`,
		"bad default":        `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"int","default":"много"}]}`,
		"no fields":          `{"name":"x","version":1,"label":"X","fields":[]}`,
		"bad name":           `{"name":"X-1","version":1,"label":"X","fields":[{"name":"a","type":"string"}]}`,
		"zero version":       `{"name":"x","version":0,"label":"X","fields":[{"name":"a","type":"string"}]}`,
		"migration future":   `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"string"}],"migrations":[{"to_version":2,"ops":[]}]}`,
		"bad pattern":        `{"name":"x","version":1,"label":"X","fields":[{"name":"a","type":"string","pattern":"("}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseType([]byte(raw)); !errors.Is(err, ErrSchema) {
				t.Fatalf("ожидалась ErrSchema, получили %v", err)
			}
		})
	}
}

func TestRegistryRejectsDuplicatePrefix(t *testing.T) {
	a := mustType(t, `{"name":"a","version":1,"label":"A","url_prefix":"x","fields":[{"name":"f","type":"string"}]}`)
	b := mustType(t, `{"name":"b","version":1,"label":"B","url_prefix":"x","fields":[{"name":"f","type":"string"}]}`)
	if _, err := NewRegistry(a, b); err == nil || !strings.Contains(err.Error(), "url_prefix") {
		t.Fatalf("ожидалась ошибка общего url_prefix, получили %v", err)
	}
}

func TestLoadRegistryProjectSchemas(t *testing.T) {
	reg, err := LoadRegistry("../../content_types")
	if err != nil {
		t.Fatalf("схемы проекта не загружаются: %v", err)
	}
	if n := len(reg.List()); n < 4 {
		t.Fatalf("по заданию нужно не меньше 4 типов, загружено %d", n)
	}
	if _, ok := reg.ByPrefix(""); !ok {
		t.Error("нет типа для корневых страниц")
	}
}

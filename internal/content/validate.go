package content

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Форматы хранения дат в data.
const (
	DateLayout     = "2006-01-02"
	DateTimeLayout = "2006-01-02T15:04"
)

// FieldErrors — ошибки валидации «поле → сообщение».
type FieldErrors map[string]string

func (e FieldErrors) Error() string {
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + e[k]
	}
	return "ошибки валидации: " + strings.Join(parts, "; ")
}

// Validate приводит данные к типам полей схемы и проверяет правила.
// Неизвестные поля отбрасываются, пустые необязательные — не сохраняются,
// bool без значения считается false (так ведёт себя неотмеченный checkbox).
func (t *Type) Validate(data map[string]any) (map[string]any, FieldErrors) {
	clean := map[string]any{}
	errs := FieldErrors{}
	for i := range t.Fields {
		f := &t.Fields[i]
		raw, present := data[f.Name]
		if present && isEmpty(raw) {
			present = false
		}
		if !present && f.Default != nil {
			raw, present = f.Default, true
		}
		if !present {
			if f.Type == FieldBool {
				clean[f.Name] = false
				continue
			}
			if f.Required {
				errs[f.Name] = "обязательное поле"
			}
			continue
		}
		v, msg := f.coerce(raw)
		if msg != "" {
			errs[f.Name] = msg
			continue
		}
		clean[f.Name] = v
	}
	// Межполевые правила проверяем после приведения, когда значения уже нормализованы.
	for _, f := range t.Fields {
		if f.After == "" {
			continue
		}
		v, ok1 := clean[f.Name].(string)
		base, ok2 := clean[f.After].(string)
		if ok1 && ok2 && v < base {
			other, _ := t.Field(f.After)
			errs[f.Name] = fmt.Sprintf("не раньше поля «%s»", other.Label)
		}
	}
	if len(errs) == 0 {
		return clean, nil
	}
	return clean, errs
}

// Normalize приводит уже сохранённые данные к типам схемы (например, числа из
// JSON, которые декодируются как float64). Ошибки игнорируются: данные в БД уже
// прошли валидацию при записи.
func (t *Type) Normalize(data map[string]any) map[string]any {
	clean, _ := t.Validate(data)
	return clean
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	}
	return false
}

// coerce приводит одно значение к типу поля и проверяет правила поля.
// Возвращает сообщение об ошибке или пустую строку.
func (f *Field) coerce(raw any) (any, string) {
	switch f.Type {
	case FieldString, FieldText, FieldMarkdown:
		s, ok := raw.(string)
		if !ok {
			return nil, "ожидается строка"
		}
		if f.Type == FieldString {
			s = strings.TrimSpace(s)
		} else {
			s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), " \n\t")
		}
		n := utf8.RuneCountInString(s)
		if f.MinLength != nil && n < *f.MinLength {
			return nil, fmt.Sprintf("не короче %d символов", *f.MinLength)
		}
		if f.MaxLength != nil && n > *f.MaxLength {
			return nil, fmt.Sprintf("не длиннее %d символов", *f.MaxLength)
		}
		if f.re != nil && !f.re.MatchString(s) {
			return nil, "не соответствует формату"
		}
		return s, ""

	case FieldInt, FieldMedia:
		n, ok := toInt(raw)
		if !ok {
			return nil, "ожидается целое число"
		}
		if f.Type == FieldMedia && n <= 0 {
			return nil, "некорректный идентификатор файла"
		}
		if f.Min != nil && n < *f.Min {
			return nil, fmt.Sprintf("не меньше %d", *f.Min)
		}
		if f.Max != nil && n > *f.Max {
			return nil, fmt.Sprintf("не больше %d", *f.Max)
		}
		return n, ""

	case FieldBool:
		switch x := raw.(type) {
		case bool:
			return x, ""
		case string:
			switch strings.ToLower(strings.TrimSpace(x)) {
			case "1", "true", "on", "yes":
				return true, ""
			case "0", "false", "off", "no":
				return false, ""
			}
		}
		return nil, "ожидается да/нет"

	case FieldDate, FieldDateTime:
		s, ok := raw.(string)
		if !ok {
			return nil, "ожидается дата"
		}
		t, ok := parseTemporal(strings.TrimSpace(s), f.Type)
		if !ok {
			if f.Type == FieldDate {
				return nil, "дата в формате ГГГГ-ММ-ДД"
			}
			return nil, "дата и время в формате ГГГГ-ММ-ДДTЧЧ:ММ"
		}
		if f.Type == FieldDate {
			return t.Format(DateLayout), ""
		}
		return t.Format(DateTimeLayout), ""

	case FieldSelect:
		s, ok := raw.(string)
		if !ok {
			return nil, "ожидается строка"
		}
		s = strings.TrimSpace(s)
		for _, o := range f.Options {
			if o == s {
				return s, ""
			}
		}
		return nil, "значение не из списка"

	case FieldURL:
		s, ok := raw.(string)
		if !ok {
			return nil, "ожидается адрес"
		}
		s = strings.TrimSpace(s)
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, "адрес должен начинаться с http:// или https://"
		}
		return s, ""
	}
	return nil, "неизвестный тип поля"
}

func toInt(raw any) (int64, bool) {
	switch x := raw.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		if x != math.Trunc(x) || math.Abs(x) > 1<<53 {
			return 0, false
		}
		return int64(x), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func parseTemporal(s, fieldType string) (time.Time, bool) {
	layouts := []string{DateTimeLayout, "2006-01-02T15:04:05", time.RFC3339, "2006-01-02 15:04"}
	if fieldType == FieldDate {
		layouts = []string{DateLayout}
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

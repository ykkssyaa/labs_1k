// Package content — ядро CMS: схемы типов материалов, валидация, жизненный цикл,
// ревизии и таксономия. Ядро не знает конкретных типов: всё берётся из схем.
package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Типы полей схемы.
const (
	FieldString   = "string"
	FieldText     = "text"
	FieldMarkdown = "markdown"
	FieldInt      = "int"
	FieldBool     = "bool"
	FieldDate     = "date"
	FieldDateTime = "datetime"
	FieldSelect   = "select"
	FieldURL      = "url"
	FieldMedia    = "media"
)

var knownFieldTypes = map[string]bool{
	FieldString: true, FieldText: true, FieldMarkdown: true, FieldInt: true, FieldBool: true,
	FieldDate: true, FieldDateTime: true, FieldSelect: true, FieldURL: true, FieldMedia: true,
}

// ReservedPrefixes — первые сегменты адресов, занятые системными маршрутами (spec/05-public.md, 4.3).
var ReservedPrefixes = map[string]bool{
	"admin": true, "api": true, "static": true, "uploads": true, "section": true, "tag": true, "search": true,
	"healthz": true,
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var prefixRe = regexp.MustCompile(`^[a-z0-9-]*$`)

type Field struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Help      string   `json:"help,omitempty"`
	Required  bool     `json:"required,omitempty"`
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	Min       *int64   `json:"min,omitempty"`
	Max       *int64   `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Options   []string `json:"options,omitempty"`
	After     string   `json:"after,omitempty"`
	Default   any      `json:"default,omitempty"`

	re *regexp.Regexp
}

// MigrationOp — шаг миграции данных при смене версии схемы.
type MigrationOp struct {
	Op      string `json:"op"` // add_field | rename_field | remove_field
	Field   string `json:"field"`
	To      string `json:"to,omitempty"`
	Default any    `json:"default,omitempty"`
}

type Migration struct {
	ToVersion int           `json:"to_version"`
	Ops       []MigrationOp `json:"ops"`
}

// Type — схема типа материала.
type Type struct {
	Name        string      `json:"name"`
	Version     int         `json:"version"`
	Label       string      `json:"label"`
	LabelPlural string      `json:"label_plural"`
	URLPrefix   string      `json:"url_prefix"`
	Fields      []Field     `json:"fields"`
	Migrations  []Migration `json:"migrations"`
}

// Field возвращает описание поля по имени.
func (t *Type) Field(name string) (*Field, bool) {
	for i := range t.Fields {
		if t.Fields[i].Name == name {
			return &t.Fields[i], true
		}
	}
	return nil, false
}

// Path — путь к материалу этого типа на сайте.
func (t *Type) Path(slug string) string {
	if t.URLPrefix == "" {
		return "/" + slug
	}
	return "/" + t.URLPrefix + "/" + slug
}

// ErrSchema — схема типа некорректна.
var ErrSchema = errors.New("некорректная схема типа")

// ParseType разбирает и проверяет одну схему.
func ParseType(raw []byte) (*Type, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields() // опечатка в имени правила не должна молча игнорироваться
	var t Type
	if err := dec.Decode(&t); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchema, err)
	}
	if err := t.check(); err != nil {
		return nil, fmt.Errorf("%w %q: %v", ErrSchema, t.Name, err)
	}
	return &t, nil
}

func (t *Type) check() error {
	if !nameRe.MatchString(t.Name) {
		return fmt.Errorf("имя типа %q: только латиница, цифры и _", t.Name)
	}
	if t.Version < 1 {
		return fmt.Errorf("version должна быть ≥ 1")
	}
	if t.Label == "" {
		return fmt.Errorf("пустой label")
	}
	if t.LabelPlural == "" {
		t.LabelPlural = t.Label
	}
	if !prefixRe.MatchString(t.URLPrefix) {
		return fmt.Errorf("url_prefix %q: только a-z, 0-9 и -", t.URLPrefix)
	}
	if ReservedPrefixes[t.URLPrefix] {
		return fmt.Errorf("url_prefix %q зарезервирован системой", t.URLPrefix)
	}
	if len(t.Fields) == 0 {
		return fmt.Errorf("нет полей")
	}
	seen := map[string]bool{}
	for i := range t.Fields {
		f := &t.Fields[i]
		if !nameRe.MatchString(f.Name) {
			return fmt.Errorf("поле %q: недопустимое имя", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("поле %q объявлено дважды", f.Name)
		}
		seen[f.Name] = true
		if !knownFieldTypes[f.Type] {
			return fmt.Errorf("поле %q: неизвестный тип %q", f.Name, f.Type)
		}
		if f.Label == "" {
			f.Label = f.Name
		}
		if f.Type == FieldSelect && len(f.Options) == 0 {
			return fmt.Errorf("поле %q: у select нет options", f.Name)
		}
		if f.Pattern != "" {
			re, err := regexp.Compile("^(?:" + f.Pattern + ")$")
			if err != nil {
				return fmt.Errorf("поле %q: pattern: %v", f.Name, err)
			}
			f.re = re
		}
	}
	for _, f := range t.Fields {
		if f.After == "" {
			continue
		}
		other, ok := t.Field(f.After)
		if !ok {
			return fmt.Errorf("поле %q: after ссылается на несуществующее поле %q", f.Name, f.After)
		}
		if !isTemporal(f.Type) || !isTemporal(other.Type) {
			return fmt.Errorf("поле %q: after допустим только между полями date/datetime", f.Name)
		}
	}
	for _, f := range t.Fields {
		if f.Default == nil {
			continue
		}
		if _, msg := f.coerce(f.Default); msg != "" {
			return fmt.Errorf("поле %q: default: %s", f.Name, msg)
		}
	}
	prev := 0
	for _, m := range t.Migrations {
		if m.ToVersion <= prev || m.ToVersion > t.Version {
			return fmt.Errorf("migrations: to_version %d вне порядка или больше version", m.ToVersion)
		}
		prev = m.ToVersion
		for _, op := range m.Ops {
			switch op.Op {
			case "add_field", "remove_field":
			case "rename_field":
				if op.To == "" {
					return fmt.Errorf("migrations: rename_field %q без to", op.Field)
				}
			default:
				return fmt.Errorf("migrations: неизвестная операция %q", op.Op)
			}
		}
	}
	return nil
}

func isTemporal(fieldType string) bool { return fieldType == FieldDate || fieldType == FieldDateTime }

// Registry — набор загруженных типов.
type Registry struct {
	types map[string]*Type
}

// NewRegistry собирает реестр из уже разобранных типов (для тестов и LoadRegistry).
func NewRegistry(types ...*Type) (*Registry, error) {
	r := &Registry{types: map[string]*Type{}}
	prefixes := map[string]string{}
	for _, t := range types {
		if _, dup := r.types[t.Name]; dup {
			return nil, fmt.Errorf("%w: тип %q объявлен дважды", ErrSchema, t.Name)
		}
		if other, dup := prefixes[t.URLPrefix]; dup {
			return nil, fmt.Errorf("%w: url_prefix %q занят типами %q и %q", ErrSchema, t.URLPrefix, other, t.Name)
		}
		prefixes[t.URLPrefix] = t.Name
		r.types[t.Name] = t
	}
	return r, nil
}

// LoadRegistry читает все схемы *.json из каталога.
func LoadRegistry(dir string) (*Registry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: в %s нет схем типов", ErrSchema, dir)
	}
	var types []*Type
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		t, err := ParseType(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		types = append(types, t)
	}
	return NewRegistry(types...)
}

func (r *Registry) Get(name string) (*Type, bool) {
	t, ok := r.types[name]
	return t, ok
}

// ByPrefix находит тип по префиксу URL ("" — тип корневых страниц).
func (r *Registry) ByPrefix(prefix string) (*Type, bool) {
	for _, t := range r.types {
		if t.URLPrefix == prefix {
			return t, true
		}
	}
	return nil, false
}

// List — все типы, отсортированные по названию.
func (r *Registry) List() []*Type {
	out := make([]*Type, 0, len(r.types))
	for _, t := range r.types {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

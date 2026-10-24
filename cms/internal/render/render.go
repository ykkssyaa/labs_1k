// Package render — изолированный слой вывода: темы, наследование тем,
// макеты и partial-шаблоны поверх html/template (ADR-0003).
//
// Тема — каталог themes/<имя>/ с файлами:
//
//	theme.json          {"label": "...", "extends": "родитель", "public": true}
//	layouts/*.html      макеты, определяют шаблон "base"
//	partials/*.html     переиспользуемые куски
//	pages/<page>.html   страницы, определяют блоки "title", "content" и др.
//	static/             CSS, картинки
//
// Файлы ищутся сначала в теме, затем в её родителях; файл с тем же именем
// в дочерней теме заменяет родительский, а {{define}} в дочерней теме
// переопределяет одноимённый блок родителя.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Theme struct {
	Name    string `json:"-"`
	Label   string `json:"label"`
	Extends string `json:"extends"`
	Public  bool   `json:"public"` // доступна как тема публичного сайта
}

type Engine struct {
	dir    string
	funcs  template.FuncMap
	themes map[string]*Theme
	// Reload — перечитывать шаблоны на каждый запрос (режим разработки).
	Reload bool

	mu    sync.Mutex
	cache map[string]*template.Template
}

// New загружает описания тем из dir и проверяет цепочки наследования.
func New(dir string, funcs template.FuncMap) (*Engine, error) {
	e := &Engine{dir: dir, funcs: template.FuncMap{}, themes: map[string]*Theme{}, cache: map[string]*template.Template{}}
	for k, v := range builtinFuncs {
		e.funcs[k] = v
	}
	for k, v := range funcs {
		e.funcs[k] = v
	}
	metas, err := filepath.Glob(filepath.Join(dir, "*", "theme.json"))
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		raw, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		var t Theme
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
		t.Name = filepath.Base(filepath.Dir(m))
		e.themes[t.Name] = &t
	}
	for name := range e.themes {
		if _, err := e.chain(name); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// chain — тема и её предки, от самой темы к корню.
func (e *Engine) chain(name string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for name != "" {
		t, ok := e.themes[name]
		if !ok {
			return nil, fmt.Errorf("тема %q не найдена", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("цикл наследования тем: %v", append(out, name))
		}
		seen[name] = true
		out = append(out, name)
		name = t.Extends
	}
	return out, nil
}

// Themes — темы для публичного сайта, по имени.
func (e *Engine) Themes() []*Theme {
	var out []*Theme
	for _, t := range e.themes {
		if t.Public {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (e *Engine) HasTheme(name string) bool {
	t, ok := e.themes[name]
	return ok && t.Public
}

// resolve собирает файлы для страницы: от корневой темы к дочерней, чтобы
// определения дочерней темы разбирались последними и побеждали.
func (e *Engine) resolve(theme, page string) ([]string, error) {
	chain, err := e.chain(theme)
	if err != nil {
		return nil, err
	}
	var files []string
	pageFile := ""
	for i := len(chain) - 1; i >= 0; i-- {
		root := filepath.Join(e.dir, chain[i])
		for _, sub := range []string{"layouts", "partials"} {
			m, _ := filepath.Glob(filepath.Join(root, sub, "*.html"))
			sort.Strings(m)
			files = append(files, m...)
		}
		if p := filepath.Join(root, "pages", page+".html"); fileExists(p) {
			pageFile = p
		}
	}
	if pageFile == "" {
		return nil, fmt.Errorf("страница %q не найдена в теме %q и её родителях", page, theme)
	}
	return append(files, pageFile), nil
}

// Has — есть ли страница page в теме или её родителях.
func (e *Engine) Has(theme, page string) bool {
	chain, err := e.chain(theme)
	if err != nil {
		return false
	}
	for _, name := range chain {
		if fileExists(filepath.Join(e.dir, name, "pages", page+".html")) {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func (e *Engine) load(theme, page string) (*template.Template, error) {
	key := theme + "/" + page
	e.mu.Lock()
	defer e.mu.Unlock()
	if t, ok := e.cache[key]; ok && !e.Reload {
		return t, nil
	}
	files, err := e.resolve(theme, page)
	if err != nil {
		return nil, err
	}
	t := template.New(key).Funcs(e.funcs)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(e.dir, f)
		if _, err := t.New(filepath.ToSlash(rel)).Parse(string(raw)); err != nil {
			return nil, fmt.Errorf("шаблон %s: %w", rel, err)
		}
	}
	if t.Lookup("base") == nil {
		return nil, fmt.Errorf("в теме %q нет макета с {{define \"base\"}}", theme)
	}
	e.cache[key] = t
	return t, nil
}

// Render выполняет страницу page темы theme. Вывод сначала собирается в буфер:
// ошибка в середине шаблона не должна отдавать клиенту половину страницы.
func (e *Engine) Render(w io.Writer, theme, page string, data any) error {
	t, err := e.load(theme, page)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", data); err != nil {
		return fmt.Errorf("render %s/%s: %w", theme, page, err)
	}
	if rw, ok := w.(http.ResponseWriter); ok && rw.Header().Get("Content-Type") == "" {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, err = buf.WriteTo(w)
	return err
}

// Static отдаёт /static/<тема>/<файл> с поиском по цепочке наследования.
func (e *Engine) Static() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		theme, file := r.PathValue("theme"), r.PathValue("file")
		chain, err := e.chain(theme)
		if err != nil || file == "" || strings.Contains(file, "..") {
			http.NotFound(w, r)
			return
		}
		for _, name := range chain {
			p := filepath.Join(e.dir, name, "static", filepath.FromSlash(file))
			if fileExists(p) {
				w.Header().Set("Cache-Control", "public, max-age=3600")
				http.ServeFile(w, r, p)
				return
			}
		}
		http.NotFound(w, r)
	})
}

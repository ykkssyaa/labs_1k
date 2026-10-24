package access

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Role — роль из config/roles.json.
type Role struct {
	Name        string   `json:"-"`
	Label       string   `json:"label"`
	Types       []string `json:"types"`
	Permissions []string `json:"permissions"`
}

// Policy — матрица прав, загруженная из конфигурации.
type Policy struct {
	roles map[string]*Role
}

// StatusDraft дублирует content.StatusDraft: access не импортирует ядро.
const StatusDraft = "draft"

// ParsePolicy разбирает JSON с ролями.
func ParsePolicy(raw []byte) (*Policy, error) {
	var cfg struct {
		Roles map[string]*Role `json:"roles"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("roles: %w", err)
	}
	if len(cfg.Roles) == 0 {
		return nil, fmt.Errorf("roles: не описано ни одной роли")
	}
	for name, r := range cfg.Roles {
		if r.Label == "" {
			return nil, fmt.Errorf("roles: у роли %q нет label", name)
		}
		if len(r.Types) == 0 {
			return nil, fmt.Errorf("roles: у роли %q пустой список types (нужен хотя бы \"*\")", name)
		}
		r.Name = name
	}
	return &Policy{roles: cfg.Roles}, nil
}

// LoadPolicy читает роли из файла.
func LoadPolicy(path string) (*Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePolicy(raw)
}

// Role возвращает роль по имени.
func (p *Policy) Role(name string) (*Role, bool) {
	r, ok := p.roles[name]
	return r, ok
}

// Roles — все роли, отсортированные по имени.
func (p *Policy) Roles() []*Role {
	out := make([]*Role, 0, len(p.roles))
	for _, r := range p.roles {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Can — единственное место, где решается, разрешено ли действие (spec/04-admin.md, 3.1).
func (p *Policy) Can(actor Actor, action string, res Resource) bool {
	role, ok := p.roles[actor.Role]
	if !ok {
		return false
	}
	if res.Type != "" && !contains(role.Types, "*") && !contains(role.Types, res.Type) {
		return false
	}
	for _, perm := range role.Permissions {
		switch {
		case perm == "*" || perm == action:
			return true
		case strings.HasSuffix(perm, ".*") && strings.HasPrefix(action, strings.TrimSuffix(perm, "*")):
			return true
		case perm == action+".own":
			if res.AuthorID == actor.ID && (res.Status == "" || res.Status == StatusDraft) {
				return true
			}
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

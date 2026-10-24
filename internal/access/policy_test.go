package access

import (
	"fmt"
	"testing"
)

func projectPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy("../../config/roles.json")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

var (
	admin  = Actor{ID: 1, Role: "admin"}
	editor = Actor{ID: 2, Role: "editor"}
	author = Actor{ID: 3, Role: "author"}
	ghost  = Actor{ID: 4, Role: "intern"} // роли нет в конфиге
)

func ownDraft() Resource     { return Resource{Type: "article", AuthorID: 3, Status: "draft"} }
func ownReview() Resource    { return Resource{Type: "article", AuthorID: 3, Status: "review"} }
func ownPublished() Resource { return Resource{Type: "article", AuthorID: 3, Status: "published"} }
func foreignDraft() Resource { return Resource{Type: "article", AuthorID: 2, Status: "draft"} }
func page() Resource         { return Resource{Type: "page"} }

// Роль → действие → ресурс → разрешено ли. Матрица из spec/04-admin.md.
func TestPolicyMatrix(t *testing.T) {
	p := projectPolicy(t)
	cases := []struct {
		actor  Actor
		action string
		res    Resource
		want   bool
	}{
		{admin, "users.manage", Resource{}, true},
		{admin, "entry.create", page(), true},
		{admin, "settings.manage", Resource{}, true},
		{admin, "entry.edit", foreignDraft(), true},

		{editor, "admin.access", Resource{}, true},
		{editor, "entry.create", page(), true},
		{editor, "entry.edit", ownPublished(), true},
		{editor, "entry.transition.publish", ownReview(), true},
		{editor, "entry.transition.archive", ownPublished(), true},
		{editor, "entry.batch", Resource{}, true},
		{editor, "entry.list.any", Resource{}, true},
		{editor, "taxonomy.manage", Resource{}, true},
		{editor, "media.delete", Resource{}, true},
		{editor, "audit.view", Resource{}, true},
		{editor, "users.manage", Resource{}, false},
		{editor, "settings.manage", Resource{}, false},

		{author, "admin.access", Resource{}, true},
		{author, "entry.create", Resource{Type: "article"}, true},
		{author, "entry.create", page(), false},
		{author, "entry.edit", ownDraft(), true},
		{author, "entry.edit", ownReview(), false},
		{author, "entry.edit", ownPublished(), false},
		{author, "entry.edit", foreignDraft(), false},
		{author, "entry.delete", ownDraft(), true},
		{author, "entry.delete", foreignDraft(), false},
		{author, "entry.rollback", ownDraft(), true},
		{author, "entry.transition.submit", ownDraft(), true},
		{author, "entry.transition.submit", foreignDraft(), false},
		{author, "entry.transition.publish", ownReview(), false},
		{author, "entry.list.any", Resource{}, false},
		{author, "entry.batch", Resource{}, false},
		{author, "media.upload", Resource{}, true},
		{author, "media.delete", Resource{}, false},
		{author, "taxonomy.manage", Resource{}, false},
		{author, "stats.view", Resource{}, false},

		{ghost, "admin.access", Resource{}, false},
		{ghost, "entry.create", Resource{Type: "article"}, false},
	}
	for _, c := range cases {
		name := fmt.Sprintf("%s/%s/%s-%s-own%v", c.actor.Role, c.action, c.res.Type, c.res.Status, c.res.AuthorID == c.actor.ID)
		t.Run(name, func(t *testing.T) {
			if got := p.Can(c.actor, c.action, c.res); got != c.want {
				t.Fatalf("Can = %v, ожидали %v", got, c.want)
			}
		})
	}
}

// Новая роль добавляется только конфигурацией — без правки кода.
func TestNewRoleFromConfigOnly(t *testing.T) {
	p, err := ParsePolicy([]byte(`{"roles": {
		"corrector": {"label": "Корректор", "types": ["article", "news"],
		              "permissions": ["admin.access", "entry.edit", "entry.transition.reject"]}
	}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := Actor{ID: 9, Role: "corrector"}
	checks := []struct {
		action string
		res    Resource
		want   bool
	}{
		{"entry.edit", Resource{Type: "news", AuthorID: 3, Status: "review"}, true},
		{"entry.transition.reject", Resource{Type: "article", AuthorID: 3, Status: "review"}, true},
		{"entry.transition.publish", Resource{Type: "article", AuthorID: 3, Status: "review"}, false},
		{"entry.edit", Resource{Type: "review", AuthorID: 3, Status: "draft"}, false},
		{"entry.create", Resource{Type: "news"}, false},
	}
	for _, ch := range checks {
		if got := p.Can(c, ch.action, ch.res); got != ch.want {
			t.Errorf("corrector %s %s: %v, ожидали %v", ch.action, ch.res.Type, got, ch.want)
		}
	}
}

func TestParsePolicyRejectsBrokenConfig(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":        `{"roles": {}}`,
		"no label":     `{"roles": {"x": {"types": ["*"], "permissions": []}}}`,
		"no types":     `{"roles": {"x": {"label": "X", "permissions": []}}}`,
		"typo in key":  `{"roles": {"x": {"label": "X", "types": ["*"], "permisions": []}}}`,
		"invalid json": `{"roles": `,
	} {
		if _, err := ParsePolicy([]byte(raw)); err == nil {
			t.Errorf("%s: конфиг должен отклоняться", name)
		}
	}
}

func TestMaskDoesNotMatchSiblingPrefix(t *testing.T) {
	p, _ := ParsePolicy([]byte(`{"roles": {"x": {"label": "X", "types": ["*"], "permissions": ["entry.transition.*"]}}}`))
	x := Actor{ID: 1, Role: "x"}
	if !p.Can(x, "entry.transition.publish", Resource{}) {
		t.Fatal("маска должна покрывать entry.transition.publish")
	}
	if p.Can(x, "entry.transitions", Resource{}) || p.Can(x, "entry.edit", Resource{}) {
		t.Fatal("маска entry.transition.* не должна покрывать соседние действия")
	}
}

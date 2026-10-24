package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

func newAuth(t *testing.T) (*Service, *store.User, access.Actor) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	policy, err := access.LoadPolicy("../../config/roles.json")
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(db, policy)
	adm, err := s.CreateUser(context.Background(), access.Actor{}, UserInput{Email: "Admin@Vestnik.local", Name: "Админ", Role: "admin", Password: "secret-123"})
	if err != nil {
		t.Fatal(err)
	}
	return s, adm, access.Actor{ID: adm.ID, Name: adm.Name, Role: adm.Role}
}

func TestLoginAndLookup(t *testing.T) {
	s, adm, _ := newAuth(t)
	ctx := context.Background()
	if _, _, err := s.Login(ctx, "admin@vestnik.local", "wrong-pass"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, _, err := s.Login(ctx, "nobody@vestnik.local", "secret-123"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("несуществующий email должен давать ту же ошибку: %v", err)
	}
	token, u, err := s.Login(ctx, " ADMIN@vestnik.local ", "secret-123")
	if err != nil || u.ID != adm.ID {
		t.Fatalf("вход: %v", err)
	}
	cur, err := s.Lookup(ctx, token)
	if err != nil || cur.User.ID != adm.ID || cur.CSRF == "" {
		t.Fatalf("lookup: %v", err)
	}
	var stored string
	s.DB.QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored)
	if stored == token {
		t.Fatal("в БД должен лежать хэш токена, а не сам токен")
	}
	if err := s.Logout(ctx, token, cur); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatal("после выхода сессия должна пропасть")
	}
}

func TestExpiredSession(t *testing.T) {
	s, _, _ := newAuth(t)
	token, _, _ := s.Login(context.Background(), "admin@vestnik.local", "secret-123")
	s.Now = func() time.Time { return time.Now().Add(SessionTTL + time.Hour) }
	if _, err := s.Lookup(context.Background(), token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("истёкшая сессия принята: %v", err)
	}
}

func TestBlockedUserLosesSessions(t *testing.T) {
	s, _, adminActor := newAuth(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, adminActor, UserInput{Email: "a@x.ru", Name: "Автор", Role: "author", Password: "password1"})
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := s.Login(ctx, "a@x.ru", "password1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUser(ctx, adminActor, u.ID, UserInput{Email: "a@x.ru", Name: "Автор", Role: "author", Active: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatal("сессия заблокированного пользователя жива")
	}
	if _, _, err := s.Login(ctx, "a@x.ru", "password1"); !errors.Is(err, ErrBadCredentials) {
		t.Fatal("заблокированный пользователь вошёл")
	}
}

func TestUserManagementRequiresPermission(t *testing.T) {
	s, adm, adminActor := newAuth(t)
	ctx := context.Background()
	ed, _ := s.CreateUser(ctx, adminActor, UserInput{Email: "e@x.ru", Name: "Ред", Role: "editor", Password: "password1"})
	editor := access.Actor{ID: ed.ID, Role: "editor"}
	if _, err := s.CreateUser(ctx, editor, UserInput{Email: "z@x.ru", Name: "З", Role: "admin", Password: "password1"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("редактор создал пользователя: %v", err)
	}
	if err := s.UpdateUser(ctx, adminActor, adm.ID, UserInput{Email: adm.Email, Name: adm.Name, Role: "author", Active: true}); err == nil {
		t.Fatal("админ понизил сам себя")
	}
	var fe FormErrors
	_, err := s.CreateUser(ctx, adminActor, UserInput{Email: "bad", Name: "", Role: "boss", Password: "1"})
	if !errors.As(err, &fe) || len(fe) != 4 {
		t.Fatalf("ожидали 4 ошибки формы, получили %v", err)
	}
	if _, err := s.CreateUser(ctx, adminActor, UserInput{Email: "e@x.ru", Name: "Дубль", Role: "author", Password: "password1"}); !errors.As(err, &fe) {
		t.Fatalf("дубль email: %v", err)
	}
}

func TestCheckCSRF(t *testing.T) {
	cur := &Current{CSRF: "abc"}
	if !CheckCSRF(cur, "abc") || CheckCSRF(cur, "abd") || CheckCSRF(cur, "") || CheckCSRF(nil, "abc") {
		t.Fatal("CheckCSRF работает неверно")
	}
}

func TestAuditRecordedInSameTransaction(t *testing.T) {
	s, _, adminActor := newAuth(t)
	ctx := context.Background()
	s.CreateUser(ctx, adminActor, UserInput{Email: "q@x.ru", Name: "Кью", Role: "author", Password: "password1"})
	s.Login(ctx, "q@x.ru", "password1")
	recs, total, err := store.NewAuditRepo(s.DB).List(ctx, store.AuditFilter{Action: "user."})
	if err != nil || total != 2 { // user.create админа (системой) и автора
		t.Fatalf("user.* записей %d: %v %+v", total, err, recs)
	}
	if _, total, _ = store.NewAuditRepo(s.DB).List(ctx, store.AuditFilter{Action: "auth.login"}); total != 1 {
		t.Fatalf("вход не записан: %d", total)
	}
}

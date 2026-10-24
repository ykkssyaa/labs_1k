// Package auth — пароли, сессии, CSRF и журнал действий пользователей.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

const SessionTTL = 7 * 24 * time.Hour

var (
	ErrBadCredentials = errors.New("неверный email или пароль")
	ErrNoSession      = errors.New("сессия не найдена или истекла")
	ErrForbidden      = errors.New("недостаточно прав")
)

type Service struct {
	DB     *sql.DB
	Policy *access.Policy
	Now    func() time.Time
}

func NewService(db *sql.DB, policy *access.Policy) *Service {
	return &Service{DB: db, Policy: policy, Now: func() time.Time { return time.Now().UTC() }}
}

// Current — пользователь текущего запроса.
type Current struct {
	User *store.User
	CSRF string
}

func (c *Current) Actor() access.Actor {
	return access.Actor{ID: c.User.ID, Name: c.User.Name, Role: c.User.Role}
}

// dummyHash — настоящий bcrypt-хэш случайного пароля. Захардкоженная строка не годится:
// если она окажется невалидной, bcrypt вернёт ошибку мгновенно и тайминг выдаст email.
var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte(randomHex(16)), bcrypt.DefaultCost)
	return h
})

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashPassword — bcrypt с параметром по умолчанию.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// Login проверяет пароль и создаёт сессию. Возвращает токен для cookie.
func (s *Service) Login(ctx context.Context, email, password string) (string, *store.User, error) {
	u, err := store.NewUserRepo(s.DB).ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, store.ErrNotFound) {
		// Сравниваем с фиктивным хэшем, чтобы время ответа не выдавало существование email.
		bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return "", nil, ErrBadCredentials
	}
	if err != nil {
		return "", nil, err
	}
	if !u.Active || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, ErrBadCredentials
	}
	token := randomHex(32)
	sess := &store.Session{TokenHash: hashToken(token), UserID: u.ID, CSRF: randomHex(16), ExpiresAt: s.Now().Add(SessionTTL)}
	if err := store.NewSessionRepo(s.DB).Insert(ctx, sess); err != nil {
		return "", nil, err
	}
	store.NewSessionRepo(s.DB).DeleteExpired(ctx)
	s.Record(ctx, s.DB, access.Actor{ID: u.ID, Name: u.Name, Role: u.Role}, "auth.login", "user", u.ID, "")
	return token, u, nil
}

// Lookup находит пользователя по токену cookie.
func (s *Service) Lookup(ctx context.Context, token string) (*Current, error) {
	if token == "" {
		return nil, ErrNoSession
	}
	sess, err := store.NewSessionRepo(s.DB).Get(ctx, hashToken(token))
	if errors.Is(err, store.ErrNotFound) || (err == nil && sess.ExpiresAt.Before(s.Now())) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	u, err := store.NewUserRepo(s.DB).Get(ctx, sess.UserID)
	if err != nil || !u.Active {
		return nil, ErrNoSession
	}
	return &Current{User: u, CSRF: sess.CSRF}, nil
}

func (s *Service) Logout(ctx context.Context, token string, cur *Current) error {
	if cur != nil {
		s.Record(ctx, s.DB, cur.Actor(), "auth.logout", "user", cur.User.ID, "")
	}
	return store.NewSessionRepo(s.DB).Delete(ctx, hashToken(token))
}

// CheckCSRF сравнивает токен формы с токеном сессии за постоянное время.
func CheckCSRF(cur *Current, formToken string) bool {
	return cur != nil && subtle.ConstantTimeCompare([]byte(cur.CSRF), []byte(formToken)) == 1
}

// Record пишет действие в журнал. Реализует content.AuditSink.
func (s *Service) Record(ctx context.Context, q store.DBTX, actor access.Actor, action, object string, objectID int64, details string) error {
	name := actor.Name
	if name == "" {
		name = "система"
	}
	return store.NewAuditRepo(q).Add(ctx, &store.AuditRecord{UserID: actor.ID, UserName: name, Action: action,
		Object: object, ObjectID: objectID, Details: details})
}

// UserInput — данные формы пользователя. Пустой пароль при правке — не менять.
type UserInput struct {
	Email    string
	Name     string
	Role     string
	Password string
	Active   bool
}

func (s *Service) checkUser(in *UserInput, creating bool) error {
	errs := map[string]string{}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if _, err := mail.ParseAddress(in.Email); err != nil || !strings.Contains(in.Email, "@") {
		errs["email"] = "некорректный email"
	}
	if n := utf8.RuneCountInString(in.Name); n == 0 || n > 100 {
		errs["name"] = "имя обязательно, до 100 символов"
	}
	if _, ok := s.Policy.Role(in.Role); !ok {
		errs["role"] = "неизвестная роль"
	}
	if (creating || in.Password != "") && utf8.RuneCountInString(in.Password) < 8 {
		errs["password"] = "пароль не короче 8 символов"
	}
	if len(errs) > 0 {
		return FormErrors(errs)
	}
	return nil
}

// FormErrors — ошибки полей формы пользователя.
type FormErrors map[string]string

func (e FormErrors) Error() string {
	return fmt.Sprintf("ошибки в форме: %v", map[string]string(e))
}

// CreateUser создаёт пользователя. actor с нулевым ID — системная команда (adduser, seed).
func (s *Service) CreateUser(ctx context.Context, actor access.Actor, in UserInput) (*store.User, error) {
	if actor.ID != 0 && !s.Policy.Can(actor, "users.manage", access.Resource{}) {
		return nil, ErrForbidden
	}
	if err := s.checkUser(&in, true); err != nil {
		return nil, err
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u := &store.User{Email: in.Email, Name: in.Name, Role: in.Role, PasswordHash: hash, Active: true}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := store.NewUserRepo(tx).Insert(ctx, u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, FormErrors{"email": "пользователь с таким email уже есть"}
		}
		return nil, err
	}
	if err := s.Record(ctx, tx, actor, "user.create", "user", u.ID, u.Email+" ("+u.Role+")"); err != nil {
		return nil, err
	}
	return u, tx.Commit()
}

// UpdateUser меняет роль, имя, активность и (если задан) пароль. Блокировка закрывает сессии.
func (s *Service) UpdateUser(ctx context.Context, actor access.Actor, id int64, in UserInput) error {
	if !s.Policy.Can(actor, "users.manage", access.Resource{}) {
		return ErrForbidden
	}
	if err := s.checkUser(&in, false); err != nil {
		return err
	}
	if id == actor.ID && (!in.Active || in.Role != actor.Role) {
		return FormErrors{"role": "нельзя заблокировать себя или сменить себе роль"}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	users := store.NewUserRepo(tx)
	u, err := users.Get(ctx, id)
	if err != nil {
		return err
	}
	u.Email, u.Name, u.Role, u.Active = in.Email, in.Name, in.Role, in.Active
	if in.Password != "" {
		if u.PasswordHash, err = HashPassword(in.Password); err != nil {
			return err
		}
	}
	if err := users.Update(ctx, u); err != nil {
		return err
	}
	if !u.Active || in.Password != "" {
		if err := store.NewSessionRepo(tx).DeleteForUser(ctx, u.ID); err != nil {
			return err
		}
	}
	if err := s.Record(ctx, tx, actor, "user.update", "user", u.ID, fmt.Sprintf("%s, роль %s, активен %v", u.Email, u.Role, u.Active)); err != nil {
		return err
	}
	return tx.Commit()
}

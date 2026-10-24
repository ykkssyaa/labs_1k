// Package analytics — сбор просмотров и агрегаты для экрана статистики (spec/06-analytics-release.md).
package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

// DayOffset — сдвиг для московских суток в SQLite: date(created_at, '+3 hours').
// В Москве нет перехода на летнее время с 2014 года, поэтому сдвиг постоянный.
const DayOffset = "+3 hours"

// Moscow — тот же сдвиг для расчётов в Go.
var Moscow = time.FixedZone("MSK", 3*60*60)

// DedupeWindow — повторный просмотр того же материала тем же посетителем в этом окне не считается.
const DedupeWindow = 30 * time.Minute

var botMarkers = []string{"bot", "crawler", "spider", "curl", "wget", "preview", "headless", "python-requests"}

type Tracker struct {
	DB      *sql.DB
	Now     func() time.Time
	secret  []byte
	baseURL string
}

// NewTracker загружает секрет для хэширования посетителей из настроек (создаёт при первом запуске).
func NewTracker(ctx context.Context, db *sql.DB, baseURL string) (*Tracker, error) {
	settings := store.NewSettingsRepo(db)
	secret, err := settings.Get(ctx, "analytics_secret", "")
	if err != nil {
		return nil, err
	}
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
		if err := settings.Set(ctx, "analytics_secret", secret); err != nil {
			return nil, err
		}
	}
	return &Tracker{DB: db, Now: func() time.Time { return time.Now().UTC() }, secret: []byte(secret), baseURL: baseURL}, nil
}

// IsBot — запрос от робота или предзагрузка браузера.
func IsBot(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	if ua == "" || r.Header.Get("Purpose") == "prefetch" || r.Header.Get("Sec-Purpose") == "prefetch" {
		return true
	}
	for _, m := range botMarkers {
		if strings.Contains(ua, m) {
			return true
		}
	}
	return false
}

// Visitor — обезличенный идентификатор посетителя на один день: HMAC(секрет, день|IP|UA).
// Сырые IP и User-Agent не хранятся, а смена дня меняет хэш.
func (t *Tracker) Visitor(r *http.Request, now time.Time) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(now.In(Moscow).Format("2006-01-02") + "|" + ip + "|" + r.UserAgent()))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

func (t *Tracker) referrerHost(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host == "" {
		return ""
	}
	if base, err := url.Parse(t.baseURL); err == nil && strings.EqualFold(base.Host, ref.Host) {
		return "" // переходы внутри сайта не интересны как источник
	}
	return strings.ToLower(ref.Host)
}

// Track записывает просмотр материала. Возвращает true, если просмотр засчитан.
func (t *Tracker) Track(ctx context.Context, r *http.Request, entryID int64) (bool, error) {
	if IsBot(r) {
		return false, nil
	}
	now := t.Now()
	visitor := t.Visitor(r, now)
	var recent int
	err := t.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM page_views WHERE entry_id = ? AND visitor = ? AND created_at > ?`,
		entryID, visitor, now.Add(-DedupeWindow).Format(store.TimeFormat)).Scan(&recent)
	if err != nil || recent > 0 {
		return false, err
	}
	_, err = t.DB.ExecContext(ctx, `INSERT INTO page_views (entry_id, visitor, referrer_host, created_at) VALUES (?, ?, ?, ?)`,
		entryID, visitor, t.referrerHost(r), now.Format(store.TimeFormat))
	return err == nil, err
}

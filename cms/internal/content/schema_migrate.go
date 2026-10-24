package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ykkssyaa/vestnik-cms/internal/access"
	"github.com/ykkssyaa/vestnik-cms/internal/store"
)

var ErrSchemaDowngrade = errors.New("версия схемы в файле ниже, чем в БД")

// systemActor — от его имени пишутся ревизии, созданные миграциями схем.
var systemActor = access.Actor{ID: 0, Name: "система"}

// SyncResult — что сделала синхронизация для одного типа.
type SyncResult struct {
	Type     string
	From, To int
	Affected int
}

// Sync сверяет версии схем с БД и применяет шаги миграций данных.
// Всё выполняется в одной транзакции: либо все типы синхронизированы, либо ничего.
func (s *Service) Sync(ctx context.Context) ([]SyncResult, error) {
	var results []SyncResult
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, typ := range s.Types.List() {
			res, err := s.syncType(ctx, tx, typ)
			if err != nil {
				return fmt.Errorf("тип %s: %w", typ.Name, err)
			}
			if res.From != res.To {
				results = append(results, res)
			}
		}
		return nil
	})
	return results, err
}

func (s *Service) syncType(ctx context.Context, tx *sql.Tx, typ *Type) (SyncResult, error) {
	res := SyncResult{Type: typ.Name, To: typ.Version}
	schema, _ := json.Marshal(typ)
	now := s.Now().Format(store.TimeFormat)
	err := tx.QueryRowContext(ctx, `SELECT version FROM content_types WHERE name = ?`, typ.Name).Scan(&res.From)
	if errors.Is(err, sql.ErrNoRows) {
		// Новый тип: материалов ещё нет, мигрировать нечего.
		res.From = typ.Version
		_, err = tx.ExecContext(ctx, `INSERT INTO content_types (name, version, schema, updated_at) VALUES (?, ?, ?, ?)`,
			typ.Name, typ.Version, string(schema), now)
		return res, err
	}
	if err != nil {
		return res, err
	}
	if res.From > typ.Version {
		return res, fmt.Errorf("%w: %d < %d", ErrSchemaDowngrade, typ.Version, res.From)
	}
	if res.From == typ.Version {
		_, err = tx.ExecContext(ctx, `UPDATE content_types SET schema = ? WHERE name = ?`, string(schema), typ.Name)
		return res, err
	}

	repo := store.NewEntryRepo(tx)
	entries, _, err := repo.List(ctx, store.EntryFilter{Type: typ.Name, Limit: 1 << 30})
	if err != nil {
		return res, err
	}
	changed := map[int64]*Entry{}
	for _, m := range typ.Migrations {
		if m.ToVersion <= res.From {
			continue
		}
		for _, op := range m.Ops {
			n := 0
			for _, e := range entries {
				if applyOp(typ, op, e.Data) {
					changed[e.ID] = e
					n++
				}
			}
			opJSON, _ := json.Marshal(op)
			if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (type_name, to_version, op, affected, applied_at)
				VALUES (?, ?, ?, ?, ?)`, typ.Name, m.ToVersion, string(opJSON), n, now); err != nil {
				return res, err
			}
		}
	}
	for _, e := range entries {
		if changed[e.ID] == nil {
			continue
		}
		e.Search = searchText(typ, e)
		e.UpdatedAt = s.Now()
		if err := repo.Update(ctx, e); err != nil {
			return res, err
		}
		if err := s.loadTermsQ(ctx, store.NewTermRepo(tx), e); err != nil {
			return res, err
		}
		if err := s.addRevision(ctx, tx, systemActor, e, fmt.Sprintf("миграция схемы v%d", typ.Version)); err != nil {
			return res, err
		}
	}
	res.Affected = len(changed)
	_, err = tx.ExecContext(ctx, `UPDATE content_types SET version = ?, schema = ?, updated_at = ? WHERE name = ?`,
		typ.Version, string(schema), now, typ.Name)
	return res, err
}

// applyOp меняет data одного материала. Возвращает true, если что-то изменилось.
func applyOp(typ *Type, op MigrationOp, data map[string]any) bool {
	switch op.Op {
	case "add_field":
		if _, exists := data[op.Field]; exists {
			return false
		}
		def := op.Default
		if def == nil {
			if f, ok := typ.Field(op.Field); ok {
				def = f.Default
			}
		}
		if def == nil {
			return false
		}
		data[op.Field] = def
		return true
	case "rename_field":
		v, exists := data[op.Field]
		if !exists {
			return false
		}
		delete(data, op.Field)
		data[op.To] = v
		return true
	case "remove_field":
		if _, exists := data[op.Field]; !exists {
			return false
		}
		delete(data, op.Field)
		return true
	}
	return false
}

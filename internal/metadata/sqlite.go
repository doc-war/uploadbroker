package metadata

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Record struct {
	Key        string
	MIMEType   string
	Size       int64
	Backend    string
	StorageKey string
	CreatedAt  int64
	ExpireAt   int64
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	query := `CREATE TABLE IF NOT EXISTS objects (
		key         TEXT PRIMARY KEY,
		mime_type   TEXT NOT NULL,
		size        INTEGER NOT NULL,
		backend     TEXT NOT NULL DEFAULT 'local',
		storage_key TEXT NOT NULL,
		created_at  INTEGER NOT NULL,
		expire_at   INTEGER NOT NULL
	)`
	if _, err := s.db.Exec(query); err != nil {
		return err
	}
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_expire_at ON objects(expire_at)"); err != nil {
		return err
	}
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_storage_key ON objects(storage_key)"); err != nil {
		return err
	}
	return nil
}

func (s *Store) Health() error {
	return s.db.Ping()
}

func (s *Store) Insert(rec *Record) error {
	query := `INSERT OR REPLACE INTO objects (key, mime_type, size, backend, storage_key, created_at, expire_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, rec.Key, rec.MIMEType, rec.Size, rec.Backend, rec.StorageKey, rec.CreatedAt, rec.ExpireAt)
	return err
}

func (s *Store) Get(key string) (*Record, error) {
	query := `SELECT key, mime_type, size, backend, storage_key, created_at, expire_at FROM objects WHERE key = ?`
	row := s.db.QueryRow(query, key)
	rec := &Record{}
	err := row.Scan(&rec.Key, &rec.MIMEType, &rec.Size, &rec.Backend, &rec.StorageKey, &rec.CreatedAt, &rec.ExpireAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Store) Delete(key string) error {
	_, err := s.db.Exec("DELETE FROM objects WHERE key = ?", key)
	return err
}

func (s *Store) ListExpired(now int64) ([]*Record, error) {
	query := `SELECT key, mime_type, size, backend, storage_key, created_at, expire_at FROM objects WHERE expire_at < ?`
	rows, err := s.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*Record
	for rows.Next() {
		rec := &Record{}
		if err := rows.Scan(&rec.Key, &rec.MIMEType, &rec.Size, &rec.Backend, &rec.StorageKey, &rec.CreatedAt, &rec.ExpireAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (s *Store) DeleteByStorageKeyPrefix(backend, prefix string) error {
	_, err := s.db.Exec("DELETE FROM objects WHERE backend = ? AND storage_key LIKE ?", backend, prefix+"%")
	return err
}

func (s *Store) GetKeyTotalSize() (int64, error) {
	var total int64
	err := s.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM objects").Scan(&total)
	return total, err
}

func (s *Store) GetRecordCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM objects").Scan(&count)
	return count, err
}

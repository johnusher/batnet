// Package sqlstore is a ledger.Store in SQLite (pure Go, no cgo).
package sqlstore

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/johnusher/batnet/internal/hlc"
	"github.com/johnusher/batnet/internal/ledger"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS entries (
	origin  BLOB    NOT NULL,
	seq     INTEGER NOT NULL,
	wall    INTEGER NOT NULL,
	logical INTEGER NOT NULL,
	raw     BLOB    NOT NULL,
	PRIMARY KEY (origin, seq)
);
CREATE INDEX IF NOT EXISTS entries_clock ON entries (wall, logical, origin, seq);
CREATE TABLE IF NOT EXISTS contig (
	origin BLOB PRIMARY KEY,
	seq    INTEGER NOT NULL
);`

// Store is a ledger.Store backed by one SQLite file.
type Store struct{ db *sql.DB }

// Open opens or creates the database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; SQLite serialises anyway
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlstore: schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Put(e *ledger.Entry) (bool, error) {
	raw, err := e.MarshalBinary()
	if err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT OR IGNORE INTO entries (origin, seq, wall, logical, raw) VALUES (?, ?, ?, ?, ?)`,
		e.Key.Origin[:], int64(e.Key.Seq), e.TS.Wall, int64(e.TS.Logical), raw)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	var c int64
	err = tx.QueryRow(`SELECT seq FROM contig WHERE origin = ?`, e.Key.Origin[:]).Scan(&c)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	for {
		var one int
		err := tx.QueryRow(`SELECT 1 FROM entries WHERE origin = ? AND seq = ?`, e.Key.Origin[:], c+1).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return false, err
		}
		c++
	}
	if _, err := tx.Exec(`INSERT INTO contig (origin, seq) VALUES (?, ?) ON CONFLICT (origin) DO UPDATE SET seq = excluded.seq`,
		e.Key.Origin[:], c); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) Get(k ledger.Key) (*ledger.Entry, error) {
	var raw []byte
	err := s.db.QueryRow(`SELECT raw FROM entries WHERE origin = ? AND seq = ?`, k.Origin[:], int64(k.Seq)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ledger.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decode(raw)
}

func (s *Store) Vector() (ledger.Vector, error) {
	rows, err := s.db.Query(`SELECT origin, seq FROM contig WHERE seq > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := ledger.Vector{}
	for rows.Next() {
		var o []byte
		var seq int64
		if err := rows.Scan(&o, &seq); err != nil {
			return nil, err
		}
		var id ledger.NodeID
		copy(id[:], o)
		v[id] = uint64(seq)
	}
	return v, rows.Err()
}

func (s *Store) Range(r ledger.Range, limit int) ([]*ledger.Entry, error) {
	return s.query(`SELECT raw FROM entries WHERE origin = ? AND seq BETWEEN ? AND ? ORDER BY seq LIMIT ?`,
		r.Origin[:], int64(r.From), int64(r.To), limit)
}

func (s *Store) Top(origin ledger.NodeID) (uint64, error) {
	var top sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(seq) FROM entries WHERE origin = ?`, origin[:]).Scan(&top)
	return uint64(top.Int64), err
}

func (s *Store) Latest() (hlc.Timestamp, error) {
	var wall, logical int64
	err := s.db.QueryRow(`SELECT wall, logical FROM entries ORDER BY wall DESC, logical DESC LIMIT 1`).Scan(&wall, &logical)
	if errors.Is(err, sql.ErrNoRows) {
		return hlc.Timestamp{}, nil
	}
	return hlc.Timestamp{Wall: wall, Logical: uint32(logical)}, err
}

func (s *Store) All() ([]*ledger.Entry, error) {
	out, err := s.query(`SELECT raw FROM entries ORDER BY wall, logical, origin, seq`)
	if err == nil {
		ledger.SortEntries(out) // same tie-break as MemStore
	}
	return out, err
}

func (s *Store) query(q string, args ...any) ([]*ledger.Entry, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ledger.Entry
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		e, err := decode(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func decode(raw []byte) (*ledger.Entry, error) {
	var e ledger.Entry
	if err := e.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return &e, nil
}

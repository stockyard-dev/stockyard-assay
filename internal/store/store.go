package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ db *sql.DB }

type Suite struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url,omitempty"`
	CreatedAt string `json:"created_at"`
	TestCount int    `json:"test_count"`
	LastRun   string `json:"last_run,omitempty"`
	PassRate  float64 `json:"pass_rate"`
}

type Test struct {
	ID          string            `json:"id"`
	SuiteID     string            `json:"suite_id"`
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	ExpectCode  int               `json:"expect_code"`
	ExpectBody  string            `json:"expect_body,omitempty"`
	Position    int               `json:"position"`
	CreatedAt   string            `json:"created_at"`
}

type Run struct {
	ID        string       `json:"id"`
	SuiteID   string       `json:"suite_id"`
	Status    string       `json:"status"` // pass, fail, partial
	Passed    int          `json:"passed"`
	Failed    int          `json:"failed"`
	TotalMs   int          `json:"total_ms"`
	Results   []TestResult `json:"results,omitempty"`
	CreatedAt string       `json:"created_at"`
}

type TestResult struct {
	TestID     string `json:"test_id"`
	TestName   string `json:"test_name"`
	Status     string `json:"status"` // pass, fail, error
	StatusCode int    `json:"status_code"`
	RespTimeMs int    `json:"resp_time_ms"`
	RespBody   string `json:"resp_body,omitempty"`
	Error      string `json:"error,omitempty"`
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil { return nil, err }
	dsn := filepath.Join(dataDir, "assay.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, err }
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS suites (id TEXT PRIMARY KEY, name TEXT NOT NULL, base_url TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS tests (id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES suites(id) ON DELETE CASCADE, name TEXT NOT NULL, method TEXT DEFAULT 'GET', path TEXT DEFAULT '/', headers_json TEXT DEFAULT '{}', body TEXT DEFAULT '', expect_code INTEGER DEFAULT 200, expect_body TEXT DEFAULT '', position INTEGER DEFAULT 0, created_at TEXT DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, suite_id TEXT NOT NULL REFERENCES suites(id) ON DELETE CASCADE, status TEXT DEFAULT '', passed INTEGER DEFAULT 0, failed INTEGER DEFAULT 0, total_ms INTEGER DEFAULT 0, results_json TEXT DEFAULT '[]', created_at TEXT DEFAULT (datetime('now')))`,
		`CREATE INDEX IF NOT EXISTS idx_tests_suite ON tests(suite_id)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_suite ON runs(suite_id)`,
	} {
		if _, err := db.Exec(q); err != nil { return nil, fmt.Errorf("migrate: %w", err) }
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }
func genID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ── Suites ──

func (d *DB) hydrateSuite(s *Suite) {
	d.db.QueryRow(`SELECT COUNT(*) FROM tests WHERE suite_id=?`, s.ID).Scan(&s.TestCount)
	d.db.QueryRow(`SELECT created_at FROM runs WHERE suite_id=? ORDER BY created_at DESC LIMIT 1`, s.ID).Scan(&s.LastRun)
	var passed, total int
	d.db.QueryRow(`SELECT COALESCE(SUM(passed),0), COALESCE(SUM(passed+failed),0) FROM runs WHERE suite_id=? AND created_at=(SELECT MAX(created_at) FROM runs WHERE suite_id=?)`, s.ID, s.ID).Scan(&passed, &total)
	if total > 0 { s.PassRate = float64(passed) / float64(total) * 100 }
}

func (d *DB) CreateSuite(s *Suite) error {
	s.ID = genID(); s.CreatedAt = now()
	_, err := d.db.Exec(`INSERT INTO suites (id,name,base_url,created_at) VALUES (?,?,?,?)`, s.ID, s.Name, s.BaseURL, s.CreatedAt)
	return err
}

func (d *DB) GetSuite(id string) *Suite {
	var s Suite
	if err := d.db.QueryRow(`SELECT id,name,base_url,created_at FROM suites WHERE id=?`, id).Scan(&s.ID, &s.Name, &s.BaseURL, &s.CreatedAt); err != nil { return nil }
	d.hydrateSuite(&s); return &s
}

func (d *DB) ListSuites() []Suite {
	rows, _ := d.db.Query(`SELECT id,name,base_url,created_at FROM suites ORDER BY name ASC`)
	if rows == nil { return nil }; defer rows.Close()
	var out []Suite
	for rows.Next() {
		var s Suite; rows.Scan(&s.ID, &s.Name, &s.BaseURL, &s.CreatedAt)
		d.hydrateSuite(&s); out = append(out, s)
	}
	return out
}

func (d *DB) UpdateSuite(id string, s *Suite) error {
	_, err := d.db.Exec(`UPDATE suites SET name=?,base_url=? WHERE id=?`, s.Name, s.BaseURL, id); return err
}

func (d *DB) DeleteSuite(id string) error {
	d.db.Exec(`DELETE FROM runs WHERE suite_id=?`, id); d.db.Exec(`DELETE FROM tests WHERE suite_id=?`, id)
	_, err := d.db.Exec(`DELETE FROM suites WHERE id=?`, id); return err
}

// ── Tests ──

func (d *DB) CreateTest(t *Test) error {
	t.ID = genID(); t.CreatedAt = now()
	if t.Method == "" { t.Method = "GET" }; if t.ExpectCode <= 0 { t.ExpectCode = 200 }
	if t.Headers == nil { t.Headers = map[string]string{} }
	hj, _ := json.Marshal(t.Headers)
	_, err := d.db.Exec(`INSERT INTO tests (id,suite_id,name,method,path,headers_json,body,expect_code,expect_body,position,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.SuiteID, t.Name, t.Method, t.Path, string(hj), t.Body, t.ExpectCode, t.ExpectBody, t.Position, t.CreatedAt)
	return err
}

func (d *DB) GetTest(id string) *Test {
	var t Test; var hj string
	if err := d.db.QueryRow(`SELECT id,suite_id,name,method,path,headers_json,body,expect_code,expect_body,position,created_at FROM tests WHERE id=?`, id).Scan(&t.ID, &t.SuiteID, &t.Name, &t.Method, &t.Path, &hj, &t.Body, &t.ExpectCode, &t.ExpectBody, &t.Position, &t.CreatedAt); err != nil { return nil }
	json.Unmarshal([]byte(hj), &t.Headers); return &t
}

func (d *DB) ListTests(suiteID string) []Test {
	rows, _ := d.db.Query(`SELECT id,suite_id,name,method,path,headers_json,body,expect_code,expect_body,position,created_at FROM tests WHERE suite_id=? ORDER BY position ASC, created_at ASC`, suiteID)
	if rows == nil { return nil }; defer rows.Close()
	var out []Test
	for rows.Next() {
		var t Test; var hj string
		rows.Scan(&t.ID, &t.SuiteID, &t.Name, &t.Method, &t.Path, &hj, &t.Body, &t.ExpectCode, &t.ExpectBody, &t.Position, &t.CreatedAt)
		json.Unmarshal([]byte(hj), &t.Headers); out = append(out, t)
	}
	return out
}

func (d *DB) UpdateTest(id string, t *Test) error {
	hj, _ := json.Marshal(t.Headers)
	_, err := d.db.Exec(`UPDATE tests SET name=?,method=?,path=?,headers_json=?,body=?,expect_code=?,expect_body=?,position=? WHERE id=?`,
		t.Name, t.Method, t.Path, string(hj), t.Body, t.ExpectCode, t.ExpectBody, t.Position, id)
	return err
}

func (d *DB) DeleteTest(id string) error { _, err := d.db.Exec(`DELETE FROM tests WHERE id=?`, id); return err }

// ── Runs ──

func (d *DB) SaveRun(r *Run) error {
	r.ID = genID(); r.CreatedAt = now()
	rj, _ := json.Marshal(r.Results)
	_, err := d.db.Exec(`INSERT INTO runs (id,suite_id,status,passed,failed,total_ms,results_json,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.SuiteID, r.Status, r.Passed, r.Failed, r.TotalMs, string(rj), r.CreatedAt)
	return err
}

func (d *DB) ListRuns(suiteID string, limit int) []Run {
	if limit <= 0 { limit = 20 }
	rows, _ := d.db.Query(`SELECT id,suite_id,status,passed,failed,total_ms,results_json,created_at FROM runs WHERE suite_id=? ORDER BY created_at DESC LIMIT ?`, suiteID, limit)
	if rows == nil { return nil }; defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run; var rj string
		rows.Scan(&r.ID, &r.SuiteID, &r.Status, &r.Passed, &r.Failed, &r.TotalMs, &rj, &r.CreatedAt)
		json.Unmarshal([]byte(rj), &r.Results); out = append(out, r)
	}
	return out
}

func (d *DB) GetRun(id string) *Run {
	var r Run; var rj string
	if err := d.db.QueryRow(`SELECT id,suite_id,status,passed,failed,total_ms,results_json,created_at FROM runs WHERE id=?`, id).Scan(&r.ID, &r.SuiteID, &r.Status, &r.Passed, &r.Failed, &r.TotalMs, &rj, &r.CreatedAt); err != nil { return nil }
	json.Unmarshal([]byte(rj), &r.Results); return &r
}

type Stats struct { Suites int `json:"suites"`; Tests int `json:"tests"`; Runs int `json:"runs"` }
func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow(`SELECT COUNT(*) FROM suites`).Scan(&s.Suites)
	d.db.QueryRow(`SELECT COUNT(*) FROM tests`).Scan(&s.Tests)
	d.db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&s.Runs)
	return s
}

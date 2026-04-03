package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard-assay/internal/store"
)

type Server struct { db *store.DB; mux *http.ServeMux }

func New(db *store.DB, limits Limits) *Server {
	s := &Server{db: db, mux: http.NewServeMux(), limits: limits}
	s.mux.HandleFunc("GET /api/suites", s.listSuites)
	s.mux.HandleFunc("POST /api/suites", s.createSuite)
	s.mux.HandleFunc("GET /api/suites/{id}", s.getSuite)
	s.mux.HandleFunc("PUT /api/suites/{id}", s.updateSuite)
	s.mux.HandleFunc("DELETE /api/suites/{id}", s.deleteSuite)
	s.mux.HandleFunc("POST /api/suites/{id}/run", s.runSuite)
	s.mux.HandleFunc("GET /api/suites/{id}/runs", s.listRuns)

	s.mux.HandleFunc("GET /api/suites/{id}/tests", s.listTests)
	s.mux.HandleFunc("POST /api/suites/{id}/tests", s.createTest)
	s.mux.HandleFunc("GET /api/tests/{id}", s.getTest)
	s.mux.HandleFunc("PUT /api/tests/{id}", s.updateTest)
	s.mux.HandleFunc("DELETE /api/tests/{id}", s.deleteTest)

	s.mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	s.mux.HandleFunc("GET /api/stats", s.stats)
	s.mux.HandleFunc("GET /api/health", s.health)

	s.mux.HandleFunc("GET /ui", s.dashboard)
	s.mux.HandleFunc("GET /ui/", s.dashboard)
	s.mux.HandleFunc("GET /", s.root)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
func writeJSON(w http.ResponseWriter, code int, v any) { w.Header().Set("Content-Type","application/json"); w.WriteHeader(code); json.NewEncoder(w).Encode(v) }
func writeErr(w http.ResponseWriter, code int, msg string) { writeJSON(w, code, map[string]string{"error": msg}) }
func (s *Server) root(w http.ResponseWriter, r *http.Request) { if r.URL.Path != "/" { http.NotFound(w, r); return }; http.Redirect(w, r, "/ui", http.StatusFound) }

func (s *Server) listSuites(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"suites": orEmpty(s.db.ListSuites())}) }
func (s *Server) createSuite(w http.ResponseWriter, r *http.Request) {
	var su store.Suite; json.NewDecoder(r.Body).Decode(&su)
	if su.Name == "" { writeErr(w, 400, "name required"); return }
	s.db.CreateSuite(&su); writeJSON(w, 201, s.db.GetSuite(su.ID))
}
func (s *Server) getSuite(w http.ResponseWriter, r *http.Request) {
	su := s.db.GetSuite(r.PathValue("id")); if su == nil { writeErr(w, 404, "not found"); return }; writeJSON(w, 200, su)
}
func (s *Server) updateSuite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id"); ex := s.db.GetSuite(id); if ex == nil { writeErr(w, 404, "not found"); return }
	var su store.Suite; json.NewDecoder(r.Body).Decode(&su)
	if su.Name == "" { su.Name = ex.Name }; s.db.UpdateSuite(id, &su); writeJSON(w, 200, s.db.GetSuite(id))
}
func (s *Server) deleteSuite(w http.ResponseWriter, r *http.Request) { s.db.DeleteSuite(r.PathValue("id")); writeJSON(w, 200, map[string]string{"deleted":"ok"}) }

func (s *Server) listTests(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"tests": orEmpty(s.db.ListTests(r.PathValue("id")))}) }
func (s *Server) createTest(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id"); if s.db.GetSuite(sid) == nil { writeErr(w, 404, "suite not found"); return }
	var t store.Test; json.NewDecoder(r.Body).Decode(&t); t.SuiteID = sid
	if t.Name == "" { writeErr(w, 400, "name required"); return }
	s.db.CreateTest(&t); writeJSON(w, 201, s.db.GetTest(t.ID))
}
func (s *Server) getTest(w http.ResponseWriter, r *http.Request) {
	t := s.db.GetTest(r.PathValue("id")); if t == nil { writeErr(w, 404, "not found"); return }; writeJSON(w, 200, t)
}
func (s *Server) updateTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id"); ex := s.db.GetTest(id); if ex == nil { writeErr(w, 404, "not found"); return }
	var t store.Test; json.NewDecoder(r.Body).Decode(&t)
	if t.Name == "" { t.Name = ex.Name }; if t.Method == "" { t.Method = ex.Method }
	if t.Path == "" { t.Path = ex.Path }; if t.ExpectCode <= 0 { t.ExpectCode = ex.ExpectCode }
	if t.Headers == nil { t.Headers = ex.Headers }
	s.db.UpdateTest(id, &t); writeJSON(w, 200, s.db.GetTest(id))
}
func (s *Server) deleteTest(w http.ResponseWriter, r *http.Request) { s.db.DeleteTest(r.PathValue("id")); writeJSON(w, 200, map[string]string{"deleted":"ok"}) }

func (s *Server) runSuite(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	suite := s.db.GetSuite(sid); if suite == nil { writeErr(w, 404, "suite not found"); return }
	tests := s.db.ListTests(sid)
	run := store.Run{SuiteID: sid}
	client := &http.Client{Timeout: 10 * time.Second}
	totalStart := time.Now()
	for _, t := range tests {
		result := store.TestResult{TestID: t.ID, TestName: t.Name}
		url := suite.BaseURL + t.Path
		var bodyReader io.Reader
		if t.Body != "" { bodyReader = strings.NewReader(t.Body) }
		req, err := http.NewRequest(t.Method, url, bodyReader)
		if err != nil { result.Status = "error"; result.Error = err.Error(); run.Failed++; run.Results = append(run.Results, result); continue }
		for k, v := range t.Headers { req.Header.Set(k, v) }
		if t.Body != "" { req.Header.Set("Content-Type", "application/json") }
		start := time.Now()
		resp, err := client.Do(req)
		result.RespTimeMs = int(time.Since(start).Milliseconds())
		if err != nil { result.Status = "error"; result.Error = err.Error(); run.Failed++; run.Results = append(run.Results, result); continue }
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16)); resp.Body.Close()
		result.StatusCode = resp.StatusCode
		result.RespBody = string(body)
		if resp.StatusCode != t.ExpectCode {
			result.Status = "fail"; result.Error = fmt.Sprintf("expected %d, got %d", t.ExpectCode, resp.StatusCode); run.Failed++
		} else if t.ExpectBody != "" && !strings.Contains(string(body), t.ExpectBody) {
			result.Status = "fail"; result.Error = "body assertion failed"; run.Failed++
		} else {
			result.Status = "pass"; run.Passed++
		}
		run.Results = append(run.Results, result)
	}
	run.TotalMs = int(time.Since(totalStart).Milliseconds())
	if run.Failed == 0 { run.Status = "pass" } else if run.Passed == 0 { run.Status = "fail" } else { run.Status = "partial" }
	s.db.SaveRun(&run)
	writeJSON(w, 200, s.db.GetRun(run.ID))
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{"runs": orEmpty(s.db.ListRuns(r.PathValue("id"), 20))}) }
func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	run := s.db.GetRun(r.PathValue("id")); if run == nil { writeErr(w, 404, "not found"); return }; writeJSON(w, 200, run)
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.db.Stats()) }
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	st := s.db.Stats(); writeJSON(w, 200, map[string]any{"status":"ok","service":"assay","suites":st.Suites,"tests":st.Tests})
}
func orEmpty[T any](s []T) []T { if s == nil { return []T{} }; return s }
func init() { log.SetFlags(log.LstdFlags | log.Lshortfile) }

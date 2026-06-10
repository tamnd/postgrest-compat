// Package js_test verifies that a PostgREST-compatible server handles the exact
// HTTP wire traffic produced by @supabase/postgrest-js.
//
// Run against PostgREST:
//
//	go test ./client/js/... -v
//
// Run against dbrest:
//
//	POSTGREST_URL=http://localhost:3001 go test ./client/js/... -v
package js_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// makeAnonJWT builds a minimal HS256 JWT with role=web_anon.
func makeAnonJWT() string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pay := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"web_anon"}`))
	msg := hdr + "." + pay
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// SELECT scenarios
// ---------------------------------------------------------------------------

// S1: select all columns
func TestS1_SelectAll(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"), nil)
	r.Status(200).ArrayLen(3)
}

// S2: select specific columns — only those keys present
func TestS2_SelectColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing 'task'", i)
		}
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] has unexpected 'done'", i)
		}
	}
}

// S3: eq filter
func TestS3_FilterEq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "done", "eq.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["done"] != true {
			t.Errorf("row[%d] done=%v, want true", i, row["done"])
		}
	}
}

// S4: count=exact — Content-Range header present with total
func TestS4_CountExact(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"), harness.H_("Prefer", "count=exact"))
	r.Status(200).HasHeader("Content-Range")
	cr := r.Header("Content-Range")
	if harness.ContentRangeTotal(cr) < 0 {
		t.Errorf("Content-Range total missing or invalid: %q", cr)
	}
}

// S5: limit + offset
func TestS5_LimitOffset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "limit", "2", "offset", "0"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) > 2 {
		t.Errorf("expected at most 2 rows, got %d", len(arr))
	}
}

// S6: order ascending
func TestS6_OrderAsc(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id", "order", "id.asc"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i := 1; i < len(arr); i++ {
		prev := toFloat64(arr[i-1]["id"])
		curr := toFloat64(arr[i]["id"])
		if curr < prev {
			t.Errorf("not sorted asc at index %d: %v >= %v", i, curr, prev)
		}
	}
}

// S7: order desc nullsfirst
func TestS7_OrderDescNullsFirst(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,due", "order", "due.desc.nullsfirst"), nil)
	r.Status(200)
	_ = r.JSONArray() // just ensure it parses
}

// S8: HEAD for count
func TestS8_HeadCount(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200).HasHeader("Content-Range")
	if len(r.RawBody()) != 0 {
		t.Errorf("HEAD must have empty body")
	}
}

// S9: single row — Accept: application/vnd.pgrst.object+json
func TestS9_SingleRow(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	r.Status(200)
	obj := r.JSONObject()
	if obj["id"] == nil {
		t.Errorf("expected id field in single object")
	}
}

// S10: single row miss — 406
func TestS10_SingleRowMiss(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "id", "eq.9999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	r.Status(406)
}

// S11: maybeSingle miss — 200, empty array
func TestS11_MaybeSingleMiss(t *testing.T) {
	h := harness.New(t)
	// GET with standard Accept: application/json; client handles null from []
	r := h.Get("/todos", harness.P("select", "*", "id", "eq.9999"),
		harness.H_("Accept", "application/json"))
	r.Status(200).ArrayLen(0)
}

// S12: embedded resource — messages with persons FK
func TestS12_EmbeddedResource(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,persons(name)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["persons"]; !ok {
			t.Errorf("row[%d] missing embedded 'persons'", i)
		}
	}
}

// S13: or filter
func TestS13_OrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "or", "(id.eq.1,id.eq.2)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) != 2 {
		t.Errorf("expected 2 rows, got %d", len(arr))
	}
}

// S14: not filter
func TestS14_NotFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "done", "not.eq.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["done"] == true {
			t.Errorf("row[%d] done=true, want false", i)
		}
	}
}

// S15: in filter
func TestS15_InFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "id", "in.(1,2)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) != 2 {
		t.Errorf("expected 2 rows, got %d", len(arr))
	}
}

// S16: like filter
func TestS16_LikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "like.*tutorial*"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 match for like.*tutorial*")
	}
}

// S17: ilike filter (case-insensitive)
func TestS17_IlikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "ilike.*TUTORIAL*"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 match for ilike.*TUTORIAL*")
	}
}

// S18: full-text search (fts)
func TestS18_FullTextSearch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "fts.tutorial"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// S19: CSV output — Accept: text/csv
func TestS19_CSVOutput(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"),
		harness.H_("Accept", "text/csv"))
	r.Status(200).ContentType("text/csv")
}

// S20: range header with offset + limit.
// PostgREST returns 206 Partial Content when offset+limit+count=exact returns
// fewer rows than the total; accept both 200 and 206.
func TestS20_RangeOffset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "offset", "0", "limit", "2"),
		harness.H_("Prefer", "count=exact"))
	r.StatusIn(200, 206).HasHeader("Content-Range")
}

// ---------------------------------------------------------------------------
// INSERT scenarios
// ---------------------------------------------------------------------------

// I1: insert single row, no return
func TestI1_InsertSingle(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil, nil, map[string]any{
		"task": "test-i1-insert",
		"done": false,
	})
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-i1-insert"), nil)
	})
}

// I2: insert + select (return=representation)
func TestI2_InsertReturn(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "test-i2-insert", "done": false},
	)
	r.Status(201)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected returned rows")
	}
	if _, ok := arr[0]["id"]; !ok {
		t.Errorf("missing id in returned row")
	}
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-i2-insert"), nil)
	})
}

// I3: bulk insert with ?columns param
func TestI3_BulkInsert(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("columns", `"task","done"`)
	r := h.Post("/todos", params, nil, []map[string]any{
		{"task": "test-i3-bulk-a", "done": false},
		{"task": "test-i3-bulk-b", "done": false},
	})
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "like.test-i3-bulk-*"), nil)
	})
}

// I4: insert with missing=default (defaultToNull: false)
func TestI4_InsertMissingDefault(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "missing=default"),
		map[string]any{"task": "test-i4-default"},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-i4-default"), nil)
	})
}

// I5: insert multiple Prefer values in one header (count + missing)
func TestI5_InsertMultiplePrefer(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "missing=default,count=exact"),
		map[string]any{"task": "test-i5-multi"},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-i5-multi"), nil)
	})
}

// ---------------------------------------------------------------------------
// UPDATE scenarios
// ---------------------------------------------------------------------------

// U1: update with filter, no return
func TestU1_UpdateNoReturn(t *testing.T) {
	h := harness.New(t)
	// insert a row to update, then restore
	h.Post("/todos", nil, nil, map[string]any{"task": "test-u1-update", "done": false})
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-u1-updated"), nil)
		h.Delete("/todos", harness.P("task", "eq.test-u1-update"), nil)
	})

	r := h.Patch("/todos", harness.P("task", "eq.test-u1-update"), nil,
		map[string]any{"task": "test-u1-updated"})
	r.Status(204)
}

// U2: update + return=representation
func TestU2_UpdateReturn(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, nil, map[string]any{"task": "test-u2-update", "done": false})
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-u2-update"), nil)
		h.Delete("/todos", harness.P("task", "eq.test-u2-updated"), nil)
	})

	r := h.Patch("/todos",
		harness.P("task", "eq.test-u2-update", "select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "test-u2-updated"},
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected returned rows")
	}
}

// U3: update with count=exact
func TestU3_UpdateCount(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, nil, map[string]any{"task": "test-u3-update", "done": false})
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-u3-update"), nil)
	})

	r := h.Patch("/todos",
		harness.P("task", "eq.test-u3-update"),
		harness.H_("Prefer", "count=exact"),
		map[string]any{"done": true},
	)
	r.StatusIn(200, 204).HasHeader("Content-Range")
}

// U4: update with no filter — affects all rows (test on a clean table state)
func TestU4_UpdateNoFilter(t *testing.T) {
	h := harness.New(t)
	// This tests that the server accepts update without filter (updates all rows)
	// We use done field which doesn't change data integrity
	r := h.Patch("/todos", nil, nil, map[string]any{"done": false})
	r.StatusIn(200, 204)
	// restore: mark id=2 as done again (seed state)
	h.Patch("/todos", harness.P("id", "eq.2"), nil, map[string]any{"done": true})
}

// ---------------------------------------------------------------------------
// UPSERT scenarios
// ---------------------------------------------------------------------------

// P1: upsert merge (default)
func TestP1_UpsertMerge(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "resolution=merge-duplicates"),
		map[string]any{"task": "test-p1-upsert", "done": false},
	)
	r.StatusIn(200, 201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-p1-upsert"), nil)
	})
}

// P2: upsert ignore duplicates
func TestP2_UpsertIgnore(t *testing.T) {
	h := harness.New(t)
	// insert first
	h.Post("/todos", nil, nil, map[string]any{"task": "test-p2-upsert", "done": false})
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-p2-upsert"), nil)
	})

	// PostgREST v14 requires on_conflict param to apply ON CONFLICT DO NOTHING.
	r := h.Post("/todos", harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=ignore-duplicates"),
		map[string]any{"task": "test-p2-upsert", "done": true},
	)
	r.StatusIn(200, 201)
}

// P3: upsert with on_conflict param
func TestP3_UpsertOnConflict(t *testing.T) {
	h := harness.New(t)
	// persons has unique email
	r := h.Post("/persons",
		harness.P("on_conflict", "email"),
		harness.H_("Prefer", "resolution=merge-duplicates"),
		map[string]any{"name": "Alice Updated", "age": 31, "email": "alice@example.com"},
	)
	r.StatusIn(200, 201)
	// restore original
	t.Cleanup(func() {
		h.Patch("/persons",
			harness.P("email", "eq.alice@example.com"),
			nil,
			map[string]any{"name": "Alice", "age": 30},
		)
	})
}

// ---------------------------------------------------------------------------
// DELETE scenarios
// ---------------------------------------------------------------------------

// D1: delete with filter
func TestD1_DeleteFilter(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, nil, map[string]any{"task": "test-d1-delete", "done": false})

	r := h.Delete("/todos", harness.P("task", "eq.test-d1-delete"), nil)
	r.Status(204)
}

// D2: delete + return=representation
func TestD2_DeleteReturn(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, nil, map[string]any{"task": "test-d2-delete", "done": false})

	r := h.Delete("/todos",
		harness.P("task", "eq.test-d2-delete", "select", "id,task"),
		harness.H_("Prefer", "return=representation"),
	)
	r.StatusIn(200, 204)
}

// D3: delete with count=exact
func TestD3_DeleteCount(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, nil, map[string]any{"task": "test-d3-count", "done": false})

	r := h.Delete("/todos",
		harness.P("task", "eq.test-d3-count"),
		harness.H_("Prefer", "count=exact"),
	)
	r.StatusIn(200, 204).HasHeader("Content-Range")
}

// ---------------------------------------------------------------------------
// RPC scenarios
// ---------------------------------------------------------------------------

// R1: rpc POST with args body
func TestR1_RpcPost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/add", nil, nil, map[string]any{"a": 1, "b": 2})
	r.Status(200)
	body := strings.TrimSpace(string(r.RawBody()))
	if body != "3" {
		t.Errorf("rpc/add: got %q, want \"3\"", body)
	}
}

// R2: rpc GET with query params
func TestR2_RpcGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/add", harness.P("a", "1", "b", "2"), nil)
	r.Status(200)
	body := strings.TrimSpace(string(r.RawBody()))
	if body != "3" {
		t.Errorf("rpc/add GET: got %q, want \"3\"", body)
	}
}

// R3: rpc HEAD
func TestR3_RpcHead(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/rpc/add", harness.P("a", "1", "b", "2"), nil)
	r.Status(200)
	if len(r.RawBody()) != 0 {
		t.Errorf("HEAD must have empty body")
	}
}

// R4: rpc with count=exact
func TestR4_RpcCount(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_todos_count", nil,
		harness.H_("Prefer", "count=exact"),
		map[string]any{},
	)
	r.Status(200)
}

// R5: rpc get_person_by_name POST
func TestR5_RpcGetPersonByName(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_person_by_name", nil, nil,
		map[string]any{"name_param": "Alice"},
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 person named Alice")
	}
	if arr[0]["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", arr[0]["name"])
	}
}

// ---------------------------------------------------------------------------
// ERRORS scenarios
// ---------------------------------------------------------------------------

// E1: single() on no rows — 406 PGRST116
func TestE1_SingleNoRows(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*", "id", "eq.9999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406).ErrorCode("PGRST116")
}

// E2: single() on many rows — 406 PGRST116
func TestE2_SingleManyRows(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406).ErrorCode("PGRST116")
}

// E5: column not found — 400
func TestE5_ColumnNotFound(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "nonexistent_col_xyz"), nil)
	r.Status(400)
}

// E6: invalid operator — 400
func TestE6_InvalidOperator(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "badop.1"), nil)
	r.Status(400)
}

// ---------------------------------------------------------------------------
// Content negotiation
// ---------------------------------------------------------------------------

// CN1: geojson — Accept: application/geo+json.
// Requires PostGIS; without it PostgreSQL raises 42883 (function not found).
// PostgREST maps this to 404. Accept 200, 404, or 406.
func TestCN1_GeoJSON(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Accept", "application/geo+json"))
	r.StatusIn(200, 404, 406)
}

// CN2: explain plan — Accept: application/vnd.pgrst.plan+json
func TestCN2_Explain(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Accept", `application/vnd.pgrst.plan+json; for="application/json"; options=analyze;`))
	r.StatusIn(200, 406) // 406 if server doesn't support explain
}

// CN3: rollback — Prefer: tx=rollback
func TestCN3_Rollback(t *testing.T) {
	h := harness.New(t)
	// Insert with tx=rollback — row should not persist
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "tx=rollback"),
		map[string]any{"task": "test-cn3-rollback", "done": false},
	)
	r.StatusIn(200, 201)
	// Verify row was not actually inserted
	check := h.Get("/todos", harness.P("task", "eq.test-cn3-rollback"), nil)
	check.Status(200).ArrayLen(0)
}

// CN4: maxAffected — Prefer: handling=strict, Prefer: max-affected=1
func TestCN4_MaxAffected(t *testing.T) {
	h := harness.New(t)
	r := h.Patch("/todos", harness.P("id", "eq.1"),
		harness.H_("Prefer", "handling=strict,max-affected=1"),
		map[string]any{"done": false},
	)
	r.StatusIn(200, 204)
}

// ---------------------------------------------------------------------------
// Filter operator coverage
// ---------------------------------------------------------------------------

// F1: neq filter
func TestF1_FilterNeq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "done", "neq.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["done"] == true {
			t.Errorf("row[%d] done=true with neq.true filter", i)
		}
	}
}

// F2: gt filter
func TestF2_FilterGt(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "*", "age", "gt.29"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		age := toFloat64(row["age"])
		if age <= 29 {
			t.Errorf("row[%d] age=%v, want >29", i, age)
		}
	}
}

// F3: gte filter
func TestF3_FilterGte(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "*", "age", "gte.30"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		age := toFloat64(row["age"])
		if age < 30 {
			t.Errorf("row[%d] age=%v, want >=30", i, age)
		}
	}
}

// F4: lt filter
func TestF4_FilterLt(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "*", "age", "lt.30"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		age := toFloat64(row["age"])
		if age >= 30 {
			t.Errorf("row[%d] age=%v, want <30", i, age)
		}
	}
}

// F5: lte filter
func TestF5_FilterLte(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "*", "age", "lte.30"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		age := toFloat64(row["age"])
		if age > 30 {
			t.Errorf("row[%d] age=%v, want <=30", i, age)
		}
	}
}

// F6: is null filter
func TestF6_FilterIsNull(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "due", "is.null"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["due"] != nil {
			t.Errorf("row[%d] due=%v, want null", i, row["due"])
		}
	}
}

// F7: contains (cs) filter for arrays
func TestF7_FilterContains(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "tags", "cs.{go}"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// F8: containedBy (cd) filter for arrays
func TestF8_FilterContainedBy(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "tags", "cd.{go,sql,pets,chores,home}"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// F9: overlaps (ov) filter for arrays
func TestF9_FilterOverlaps(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "tags", "ov.{go,rust}"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// F10: match filter (multiple eq conditions)
func TestF10_FilterMatch(t *testing.T) {
	h := harness.New(t)
	// .match({done: false, id: 1}) → ?done=eq.false&id=eq.1
	params := url.Values{}
	params.Set("done", "eq.false")
	params.Set("id", "eq.1")
	params.Set("select", "*")
	r := h.Get("/todos", params, nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) != 1 {
		t.Errorf("expected 1 row, got %d", len(arr))
	}
}

// F11: filter escape hatch (raw filter)
func TestF11_FilterRaw(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "*", "name", "eq.Alice"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) != 1 {
		t.Errorf("expected 1 person, got %d", len(arr))
	}
}

// F12: like(all) — multiple patterns (likeAllOf)
func TestF12_LikeAllOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "like(all).{*a*,*o*}"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// F13: like(any) — any pattern matches (likeAnyOf)
func TestF13_LikeAnyOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "like(any).{*cat*,*laundry*}"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 match")
	}
}

// F14: ilike(all) — case-insensitive all-of match (ilikeAllOf)
func TestF14_IlikeAllOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "ilike(all).{*A*,*o*}"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// F15: ilike(any) — case-insensitive any match (ilikeAnyOf)
func TestF15_IlikeAnyOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "task", "ilike(any).{*CAT*,*LAUNDRY*}"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 match")
	}
}

// ---------------------------------------------------------------------------
// Schema switching
// ---------------------------------------------------------------------------

// SS1: GET with Accept-Profile: private
func TestSS1_SchemaGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/items", harness.P("select", "*"),
		harness.H_("Accept-Profile", "private"))
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected private.items rows")
	}
}

// SS2: GET specific columns from private schema
func TestSS2_SchemaGetColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/items", harness.P("select", "id,name"),
		harness.H_("Accept-Profile", "private"))
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 2 {
		t.Errorf("expected 2 private.items rows, got %d", len(arr))
	}
}

// SS3: HEAD on private schema
func TestSS3_SchemaHead(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/items", nil,
		harness.H_("Accept-Profile", "private"))
	r.Status(200)
}

// SS4: POST Content-Profile (write to private schema requires grant — expect 201 or 403)
func TestSS4_SchemaPost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/items", nil,
		harness.H_("Content-Profile", "private"),
		map[string]any{"name": "test-ss4-item"},
	)
	// private schema only has SELECT grant; PostgREST returns 401 for anon
	// access to a resource requiring higher privileges (prompts client to auth).
	r.StatusIn(201, 401, 403)
	t.Cleanup(func() {
		// attempt cleanup in case insert succeeded
		h.Delete("/items",
			harness.P("name", "eq.test-ss4-item"),
			harness.H_("Content-Profile", "private"),
		)
	})
}

// ---------------------------------------------------------------------------
// Transform: order / limit / range with referencedTable
// ---------------------------------------------------------------------------

// T1: order on embedded table
func TestT1_OrderEmbedded(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,messages(id,message)")
	params.Set("messages.order", "id.asc")
	r := h.Get("/channels", params, nil)
	r.Status(200)
	_ = r.JSONArray()
}

// T2: limit on embedded table
func TestT2_LimitEmbedded(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,messages(id,message)")
	params.Set("messages.limit", "1")
	r := h.Get("/channels", params, nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if msgs, ok := row["messages"]; ok {
			if msgArr, ok := msgs.([]any); ok && len(msgArr) > 1 {
				t.Errorf("row[%d] has %d embedded messages, want <=1", i, len(msgArr))
			}
		}
	}
}

// T3: range as offset + limit
func TestT3_Range(t *testing.T) {
	h := harness.New(t)
	// range(0, 1) → offset=0&limit=2
	r := h.Get("/todos", harness.P("select", "*", "offset", "0", "limit", "2"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) > 2 {
		t.Errorf("expected at most 2 rows from range, got %d", len(arr))
	}
}

// T4: order desc nullslast
func TestT4_OrderDescNullsLast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,due", "order", "due.desc.nullslast"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// ---------------------------------------------------------------------------
// Embedded resource deeper tests
// ---------------------------------------------------------------------------

// ER1: cities with countries embedded
func TestER1_CitiesCountries(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/cities", harness.P("select", "name,countries(name)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["countries"]; !ok {
			t.Errorf("row[%d] missing embedded 'countries'", i)
		}
	}
}

// ER2: persons with assignments embedded
func TestER2_PersonsAssignments(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("select", "name,assignments(todo_id)"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// ER3: filter on embedded resource
func TestER3_EmbeddedFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,messages(id,message)")
	params.Set("messages.message", "eq.hello world")
	r := h.Get("/channels", params, nil)
	r.Status(200)
	_ = r.JSONArray()
}

// ER4: or filter on embedded resource
func TestER4_EmbeddedOrFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,messages(id,message)")
	params.Set("messages.or", "(id.eq.1,id.eq.2)")
	r := h.Get("/channels", params, nil)
	r.Status(200)
	_ = r.JSONArray()
}

// ---------------------------------------------------------------------------
// Count header
// ---------------------------------------------------------------------------

// CT1: planned count
// count=planned uses EXPLAIN for the estimate; 206 is expected when the
// estimate exceeds actual rows (e.g., fresh table with stale statistics).
func TestCT1_CountPlanned(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Prefer", "count=planned"))
	r.StatusIn(200, 206)
}

// CT2: estimated count
// Same as CT1: 206 is valid when the estimate is higher than actual rows.
func TestCT2_CountEstimated(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Prefer", "count=estimated"))
	r.StatusIn(200, 206)
}

// ---------------------------------------------------------------------------
// Prefer header multi-value (js sends comma-separated in one header)
// ---------------------------------------------------------------------------

// MH1: return=representation,count=exact in single Prefer header
func TestMH1_PreferMultiValue(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation,count=exact"),
		map[string]any{"task": "test-mh1-multi", "done": false},
	)
	r.StatusIn(200, 201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-mh1-multi"), nil)
	})
}

// MH2: resolution=merge-duplicates,missing=default in single Prefer header
func TestMH2_UpsertPreferMultiValue(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "resolution=merge-duplicates,missing=default"),
		map[string]any{"task": "test-mh2-upsert", "done": false},
	)
	r.StatusIn(200, 201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.test-mh2-upsert"), nil)
	})
}

// ---------------------------------------------------------------------------
// select whitespace stripping
// ---------------------------------------------------------------------------

// WS1: whitespace stripped from select cols
func TestWS1_SelectWhitespace(t *testing.T) {
	h := harness.New(t)
	// "id, task" → "id,task" after stripping (server should accept both)
	r := h.Get("/todos", harness.P("select", "id, task"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing id", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing task", i)
		}
	}
}

// WS2: empty select → select=* (all columns)
func TestWS2_SelectEmpty(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) != 3 {
		t.Errorf("expected 3 todos, got %d", len(arr))
	}
	// Ensure all known columns are present
	for _, col := range []string{"id", "done", "task", "due", "tags"} {
		if _, ok := arr[0][col]; !ok {
			t.Errorf("row[0] missing column %q in select=*", col)
		}
	}
}

// ---------------------------------------------------------------------------
// Filter operator coverage (continued)
// ---------------------------------------------------------------------------

// F26: is.true filter
func TestF26_IsTrue(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "done", "is.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["done"] != true {
			t.Errorf("row[%d] done=%v, want true", i, row["done"])
		}
	}
}

// F27: is.false filter
func TestF27_IsFalse(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "done", "is.false"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["done"] != false {
			t.Errorf("row[%d] done=%v, want false", i, row["done"])
		}
	}
}

// F29: not.in filter
func TestF29_NotIn(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "id", "not.in.(1,2)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		id := toFloat64(row["id"])
		if id == 1 || id == 2 {
			t.Errorf("row[%d] id=%v, should not be 1 or 2", i, id)
		}
	}
}

// F30: not.eq filter
func TestF30_NotEq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "id", "not.eq.1"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if toFloat64(row["id"]) == 1 {
			t.Errorf("row[%d] id=1 should be excluded by not.eq.1", i)
		}
	}
}

// F31: not.is.null filter
func TestF31_NotIsNull(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*", "due", "not.is.null"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if row["due"] == nil {
			t.Errorf("row[%d] due=null, should be excluded by not.is.null", i)
		}
	}
}

// FR_Sl: sl range filter
func TestFR_SlFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "sl.(5,10)"), nil)
	r.StatusIn(200, 400)
}

// FR_Sr: sr range filter
func TestFR_SrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "sr.(0,0)"), nil)
	r.StatusIn(200, 400)
}

// FR_Nxl: nxl range filter
func TestFR_NxlFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "nxl.(0,5)"), nil)
	r.StatusIn(200, 400)
}

// FR_Nxr: nxr range filter
func TestFR_NxrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "nxr.(0,5)"), nil)
	r.StatusIn(200, 400)
}

// ---------------------------------------------------------------------------
// Select expression coverage
// ---------------------------------------------------------------------------

// SEL2: inner join embed
func TestSEL2_InnerJoinEmbed(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,message,persons!inner(name)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["persons"]; !ok {
			t.Errorf("row[%d] missing embedded 'persons'", i)
		}
	}
}

// SEL3: spread embed syntax
func TestSEL3_SpreadEmbed(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,...persons(name)"), nil)
	r.StatusIn(200, 400) // server may not support spread syntax
}

// SEL4: embed count
func TestSEL4_EmbedCount(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels", harness.P("select", "id,messages(count)"), nil)
	r.Status(200)
	_ = r.JSONArray()
}

// SEL5: column alias
func TestSEL5_ColumnAlias(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "task_text:task"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["task_text"]; !ok {
			t.Errorf("row[%d] missing aliased key 'task_text'", i)
		}
	}
}

// SEL6: column cast to text
func TestSEL6_ColumnCast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,done::text"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if v, ok := row["done"]; ok {
			if _, isString := v.(string); !isString {
				t.Errorf("row[%d] done cast to text: expected string, got %T (%v)", i, v, v)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// MaybeSingle POST
// ---------------------------------------------------------------------------

// MS1: POST with Accept: application/vnd.pgrst.object+json
func TestMS1_MaybeSinglePost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation", "Accept", "application/vnd.pgrst.object+json"),
		map[string]any{"task": "js-ms1-maybesingle", "done": false},
	)
	r.StatusIn(200, 201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.js-ms1-maybesingle"), nil)
	})
}

// ---------------------------------------------------------------------------
// Auth / JWT
// ---------------------------------------------------------------------------

// AU1: valid JWT bearer (role=web_anon) — should succeed as anon
func TestAU1_JWTBearerAnon(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Authorization", "Bearer "+makeAnonJWT()))
	r.Status(200)
}

// AU2: invalid JWT bearer — server may reject (401) or treat as anon (200)
func TestAU2_JWTBearerInvalid(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"),
		harness.H_("Authorization", "Bearer invalidtoken.notvalid.atall"))
	r.StatusIn(200, 401)
}

// ---------------------------------------------------------------------------
// Error scenarios (continued)
// ---------------------------------------------------------------------------

// E3: unique constraint violation
func TestE3_UniqueConstraintViolation(t *testing.T) {
	h := harness.New(t)
	// "finish tutorial" is the seed task for id=1; inserting again hits unique constraint
	r := h.Post("/todos", nil, nil, map[string]any{"task": "finish tutorial", "done": false})
	r.StatusIn(400, 409)
}

// E4: FK constraint violation
func TestE4_FKConstraintViolation(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/messages", nil, nil, map[string]any{
		"message":    "test-e4-fk",
		"channel_id": 99999,
		"person_id":  1,
	})
	r.StatusIn(400, 409, 422)
}

// E7: table not found
func TestE7_TableNotFound(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/nonexistent_table_xyz", nil, nil)
	r.Status(404)
}

// E8: rpc not found
func TestE8_RpcNotFound(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/nonexistent_func_xyz", nil, nil, map[string]any{})
	r.Status(404)
}

// ---------------------------------------------------------------------------
// Count header (continued)
// ---------------------------------------------------------------------------

// CT3: HEAD with count=exact
func TestCT3_CountHead(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200).HasHeader("Content-Range")
	if len(r.RawBody()) != 0 {
		t.Errorf("HEAD must have empty body")
	}
}

// ---------------------------------------------------------------------------
// Embedded resource with filter
// ---------------------------------------------------------------------------

// FR3: embed with filter on embedded side
func TestFR3_EmbedWithFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,persons(name)")
	params.Set("persons.name", "eq.Alice")
	r := h.Get("/messages", params, nil)
	r.Status(200)
	_ = r.JSONArray()
}

// ---------------------------------------------------------------------------
// Transaction rollback
// ---------------------------------------------------------------------------

// TX1: insert with tx=rollback, row must not persist
func TestTX1_TxRollback(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "tx=rollback,return=representation"),
		map[string]any{"task": "js-tx1-rollback-xz9m", "done": false},
	)
	r.StatusIn(200, 201)
	// Row must not be persisted
	check := h.Get("/todos", harness.P("task", "eq.js-tx1-rollback-xz9m"), nil)
	check.Status(200).ArrayLen(0)
}

// ---------------------------------------------------------------------------
// RPC (continued)
// ---------------------------------------------------------------------------

// RPC1: GET /rpc/get_todos_count (stable function via GET)
func TestRPC1_RpcGetStable(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/get_todos_count", nil, nil)
	r.Status(200)
}

// RPC2: POST /rpc/get_person_by_name?select=name
func TestRPC2_RpcWithSelect(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_person_by_name",
		harness.P("select", "name"),
		nil,
		map[string]any{"name_param": "Alice"},
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Errorf("expected at least 1 row from get_person_by_name")
	}
	if _, ok := arr[0]["name"]; !ok {
		t.Errorf("row missing 'name' field from select=name")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}


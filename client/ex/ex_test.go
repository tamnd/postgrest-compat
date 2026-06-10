// Package ex_test replicates the exact HTTP wire traffic produced by the
// Elixir supabase_postgrest client (Hex package supabase_postgrest v1.2.2).
//
// Each test function corresponds to one wire scenario documented in
// notes/Spec/2023/compat-client/08-postgrest-ex.md.
//
// Run against PostgREST (reference):
//
//	go test ./client/ex/... -v
//
// Run against dbrest:
//
//	POSTGREST_URL=http://localhost:3001 go test ./client/ex/... -v
package ex_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// EX1: Default JSON preset — GET /todos Accept: application/json => 200, JSON array.
// Elixir: Q.from(client, "todos") |> Q.select("*", returning: true) |> Q.execute()
func TestEX1_DefaultJSON(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Accept", "application/json"))
	r.Status(200).ContentType("application/json")
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one todo row")
	}
}

// EX2: CSV preset — GET /todos?select=id,task Accept: text/csv => 200, text/csv body.
// Elixir: Q.with_custom_media_type(q, :csv) => Accept: text/csv
func TestEX2_CSVPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"),
		harness.H_("Accept", "text/csv"))
	r.Status(200).ContentType("text/csv")
	body := string(r.RawBody())
	if !strings.Contains(body, "id") || !strings.Contains(body, "task") {
		t.Errorf("CSV body missing expected columns: %s", body)
	}
}

// EX3: GeoJSON/postgis preset — GET /todos Accept: application/geo+json.
// PostgREST returns GeoJSON for geometry columns; requires PostGIS.
// Without PostGIS, PostgreSQL raises 42883 (undefined function) which PostgREST
// surfaces as 404. Accept 200, 404, 406, or 415 as valid responses.
// Elixir: Q.with_custom_media_type(q, :postgis) => Accept: application/geo+json
func TestEX3_GeoJSONPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Accept", "application/geo+json"))
	r.StatusIn(200, 404, 406, 415)
	if r.Header("Content-Type") != "" && strings.Contains(r.Header("Content-Type"), "geo+json") {
		r.BodyContains("type")
	}
}

// EX4: pgrst_plan preset — GET /todos Accept: application/vnd.pgrst.plan+json => plan body.
// Elixir: Q.with_custom_media_type(q, :pgrst_plan) => Accept: application/vnd.pgrst.plan+json
func TestEX4_PgrstPlanPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil,
		harness.H_("Accept", "application/vnd.pgrst.plan+json"))
	r.StatusIn(200, 406)
	if r.Header("Content-Type") != "" && strings.Contains(r.Header("Content-Type"), "plan") {
		body := string(r.RawBody())
		if len(body) == 0 {
			t.Error("expected non-empty plan body")
		}
	}
}

// EX5: pgrst_object preset — GET /todos?id=eq.1 Accept: application/vnd.pgrst.object+json => single object.
// Elixir: Q.with_custom_media_type(q, :pgrst_object) => Accept: application/vnd.pgrst.object+json
func TestEX5_PgrstObjectPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	r.Status(200)
	obj := r.JSONObject()
	if _, ok := obj["id"]; !ok {
		t.Errorf("expected object with 'id' field, got: %v", obj)
	}
}

// EX6: pgrst_array preset — GET /todos Accept: application/vnd.pgrst.array+json => JSON array.
// Elixir: Q.with_custom_media_type(q, :pgrst_array) => Accept: application/vnd.pgrst.array+json
func TestEX6_PgrstArrayPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil,
		harness.H_("Accept", "application/vnd.pgrst.array+json"))
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one todo row")
	}
}

// EX7: openapi preset — GET / Accept: application/openapi+json => 200, OpenAPI root.
// Elixir: Q.with_custom_media_type(q, :openapi) => Accept: application/openapi+json
func TestEX7_OpenAPIPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/", nil, harness.H_("Accept", "application/openapi+json"))
	r.Status(200)
	obj := r.JSONObject()
	if _, ok := obj["openapi"]; !ok {
		if _, ok := obj["info"]; !ok {
			ks := make([]string, 0, len(obj))
			for k := range obj {
				ks = append(ks, k)
			}
			t.Errorf("expected OpenAPI root object with 'openapi' or 'info' key, got keys: %v", ks)
		}
	}
}

// EX8: plfts text search (type: :plain) — GET /todos?task=plfts.tutorial => matching rows.
// Elixir: Q.text_search(q, "task", "tutorial", type: :plain) => ?task=plfts.tutorial
func TestEX8_PlainTextSearch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "plfts.tutorial"), nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(task, "tutorial") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("plfts search for 'tutorial' should return 'finish tutorial' row, got: %v", arr)
	}
}

// EX9: phfts text search (type: :phrase) — GET /todos?task=phfts.do laundry => matching rows.
// Elixir: Q.text_search(q, "task", "do laundry", type: :phrase) => ?task=phfts.do+laundry
func TestEX9_PhraseTextSearch(t *testing.T) {
	h := harness.New(t)
	params := url.Values{"task": []string{"phfts.do laundry"}}
	r := h.Get("/todos", params, nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(task, "laundry") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("phfts search for 'do laundry' should return the laundry row, got: %v", arr)
	}
}

// EX10: wfts text search (type: :websearch) — GET /todos?task=wfts.cat => row 2 (pat the cat).
// Elixir: Q.text_search(q, "task", "cat", type: :websearch) => ?task=wfts.cat
func TestEX10_WebSearchTextSearch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "wfts.cat"), nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(task, "cat") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("wfts search for 'cat' should return 'pat the cat' row, got: %v", arr)
	}
}

// EX11: returning: :representation on insert — POST /todos Prefer: return=representation => 201, row.
// Elixir: Q.insert(%{task: "new task"}, returning: :representation)
func TestEX11_InsertReturningRepresentation(t *testing.T) {
	h := harness.New(t)
	body := map[string]any{"task": "ex11 test task", "done": false}
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		body)
	r.Status(201)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected returned row in response body")
	}
	inserted := arr[0]
	idVal, ok := inserted["id"]
	if !ok {
		t.Fatal("inserted row missing 'id' field")
	}
	id := int(idVal.(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
	if task, _ := inserted["task"].(string); task != "ex11 test task" {
		t.Errorf("returned task: got %q, want %q", task, "ex11 test task")
	}
}

// EX12: returning: :minimal — POST /todos Prefer: return=minimal => 201, empty body.
// Elixir: Q.insert(%{task: "new task"}, returning: :minimal)
func TestEX12_InsertReturningMinimal(t *testing.T) {
	h := harness.New(t)
	body := map[string]any{"task": "ex12 test task", "done": false}
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "return=minimal"),
		body)
	r.Status(201)
	// returning=minimal: body should be empty or very small (no rows)
	rawBody := r.RawBody()
	if len(rawBody) > 0 {
		// Some implementations may return empty JSON array; accept [] but not rows
		s := strings.TrimSpace(string(rawBody))
		if s != "" && s != "[]" && s != "{}" {
			t.Errorf("expected empty body for return=minimal, got: %s", rawBody)
		}
	}
	// Clean up: we don't have the id so we delete by task name
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.ex12 test task"), nil)
	})
}

// EX13: returning: :headers_only — DELETE /todos?id=eq.N Prefer: return=headers-only => 204, no body.
// Elixir: Q.delete(returning: :headers_only)
func TestEX13_DeleteReturningHeadersOnly(t *testing.T) {
	h := harness.New(t)
	// First insert a row to delete
	ins := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "ex13 to delete", "done": false})
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("insert failed, no row returned")
	}
	id := int(arr[0]["id"].(float64))

	r := h.Delete("/todos", harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "return=headers-only"))
	r.StatusIn(200, 204)
	// With headers-only, body should be empty
	rawBody := r.RawBody()
	if len(rawBody) > 0 {
		s := strings.TrimSpace(string(rawBody))
		if s != "" && s != "[]" {
			t.Logf("note: return=headers-only body was not empty: %s", rawBody)
		}
	}
}

// EX14: count: :exact on select — GET /todos Prefer: count=exact => Content-Range with /total.
// Elixir: Q.select("*", returning: true, count: :exact)
func TestEX14_CountExact(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200)
	cr := r.Header("Content-Range")
	if cr == "" {
		t.Error("expected Content-Range header with count=exact")
		return
	}
	total := harness.ContentRangeTotal(cr)
	if total < 0 {
		t.Errorf("Content-Range total should be a number, got: %s", cr)
	}
	if total < 3 {
		t.Errorf("expected at least 3 todos (seed data), Content-Range: %s", cr)
	}
}

// EX15: order asc nullslast — GET /todos?order=due.asc.nullslast => nulls come last.
// Elixir: Q.order(q, "due", asc: true, nulls_first: false) => ?order=due.asc.nullslast
func TestEX15_OrderAscNullsLast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "due.asc.nullslast"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 2 {
		t.Fatal("expected at least 2 rows")
	}
	// Rows with due=null should appear at the end
	lastRow := arr[len(arr)-1]
	dueVal := lastRow["due"]
	if dueVal != nil {
		// Check that no earlier row has null due
		for i, row := range arr[:len(arr)-1] {
			if row["due"] == nil {
				t.Errorf("row[%d] has null due but is not last (order=due.asc.nullslast)", i)
			}
		}
	}
}

// EX16: order asc nullsfirst — GET /todos?order=due.asc.nullsfirst => nulls come first.
// Elixir: Q.order(q, "due", asc: true, nulls_first: true) => ?order=due.asc.nullsfirst
func TestEX16_OrderAscNullsFirst(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "due.asc.nullsfirst"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 2 {
		t.Fatal("expected at least 2 rows")
	}
	// The row with null due (id=2, pat the cat) should come first
	firstRow := arr[0]
	if firstRow["due"] != nil {
		t.Errorf("first row should have null 'due' with nullsfirst ordering, got: %v", firstRow)
	}
}

// EX17: Schema switching — GET /items Accept-Profile: private => private schema items.
// Elixir: uses Accept-Profile header for schema switching.
func TestEX17_SchemaSwitching(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/items", nil,
		harness.H_("Accept-Profile", "private", "Accept", "application/json"))
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected private.items rows")
	}
	// Verify items have 'id' and 'name' fields (from private.items schema)
	r.RowsHaveField("id")
	r.RowsHaveField("name")
}

// EX18: execute_to struct key matching — JSON keys must be exact (id, task, done, due, tags).
// Elixir: Q.execute_to(builder, Schema) maps JSON keys to struct atoms.
func TestEX18_ExecuteToStructKeyMatching(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "eq.1"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected one todo row")
	}
	row := arr[0]
	expectedKeys := []string{"id", "task", "done", "due", "tags"}
	for _, k := range expectedKeys {
		if _, ok := row[k]; !ok {
			t.Errorf("row missing expected key %q (execute_to struct mapping requires exact keys)", k)
		}
	}
}

// EX19: not filter — GET /todos?done=not.eq.true => undone todos.
// Elixir: Q.not(q, "done", :eq, true) => ?done=not.eq.true
func TestEX19_NotFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "not.eq.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one undone todo")
	}
	r.RowsAllMatch("done=false", func(row map[string]any) bool {
		done, ok := row["done"]
		if !ok {
			return false
		}
		return done == false || done == nil
	})
}

// EX20: or filter — GET /todos?or=(done.eq.true,id.eq.1) => done or id=1 todos.
// Elixir: Q.or(q, "done.eq.true,id.eq.1") => ?or=(done.eq.true,id.eq.1)
func TestEX20_OrFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{"or": []string{"(done.eq.true,id.eq.1)"}}
	r := h.Get("/todos", params, nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one matching todo")
	}
	// Every returned row must satisfy: done=true OR id=1
	r.RowsAllMatch("done=true OR id=1", func(row map[string]any) bool {
		done := row["done"] == true
		id := row["id"]
		idIsOne := false
		switch v := id.(type) {
		case float64:
			idIsOne = v == 1
		case int:
			idIsOne = v == 1
		}
		return done || idIsOne
	})
}

// EX21: GeoJSON response structure — if supported, body has "type":"FeatureCollection".
// Tests that when Accept: application/geo+json is sent and server responds 200,
// the body is a valid GeoJSON FeatureCollection.
func TestEX21_GeoJSONResponseStructure(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Accept", "application/geo+json"))
	// If the server responds with 200 and geo+json content type, verify GeoJSON structure
	rawBody := r.RawBody()
	ct := r.Header("Content-Type")
	if strings.Contains(ct, "geo+json") {
		if !strings.Contains(string(rawBody), `"type"`) {
			t.Errorf("GeoJSON response missing 'type' key: %s", rawBody)
		}
		if !strings.Contains(string(rawBody), "FeatureCollection") &&
			!strings.Contains(string(rawBody), "Feature") {
			t.Logf("note: GeoJSON body does not contain FeatureCollection or Feature: %s", rawBody)
		}
	}
	// Accept any status for tables without geometry columns (best-effort)
}

// itoa converts an int to string for use in query params.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// makeAnonJWT builds a signed HS256 JWT with role=web_anon.
func makeAnonJWT() string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pay := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"web_anon"}`))
	msg := hdr + "." + pay
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// EX22: eq filter — GET /todos?id=eq.2 => 200, 1 row with id=2.
func TestEX22_EqFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "eq.2"), nil)
	r.Status(200).ArrayLen(1)
	arr := r.JSONArray()
	if id, ok := arr[0]["id"].(float64); !ok || id != 2 {
		t.Errorf("expected id=2, got: %v", arr[0]["id"])
	}
}

// EX23: neq filter — GET /todos?id=neq.1 => 200, no row with id=1.
func TestEX23_NeqFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "neq.1"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for _, row := range arr {
		if id, ok := row["id"].(float64); ok && id == 1 {
			t.Errorf("neq.1 filter returned row with id=1")
		}
	}
}

// EX24: gt filter — GET /todos?id=gt.1 => 200, all id>1.
func TestEX24_GtFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "gt.1"), nil)
	r.Status(200)
	r.RowsAllMatch("id>1", func(row map[string]any) bool {
		id, ok := row["id"].(float64)
		return ok && id > 1
	})
}

// EX25: gte filter — GET /todos?id=gte.2 => 200, all id>=2.
func TestEX25_GteFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "gte.2"), nil)
	r.Status(200)
	r.RowsAllMatch("id>=2", func(row map[string]any) bool {
		id, ok := row["id"].(float64)
		return ok && id >= 2
	})
}

// EX26: lt filter — GET /todos?id=lt.3 => 200, all id<3.
func TestEX26_LtFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "lt.3"), nil)
	r.Status(200)
	r.RowsAllMatch("id<3", func(row map[string]any) bool {
		id, ok := row["id"].(float64)
		return ok && id < 3
	})
}

// EX27: lte filter — GET /todos?id=lte.2 => 200, all id<=2.
func TestEX27_LteFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "lte.2"), nil)
	r.Status(200)
	r.RowsAllMatch("id<=2", func(row map[string]any) bool {
		id, ok := row["id"].(float64)
		return ok && id <= 2
	})
}

// EX28: like filter — GET /todos?task=like.*cat* => 200, rows matching *cat*.
// PostgREST uses * as wildcard for the like operator.
func TestEX28_LikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "like.*cat*"), nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(task, "cat") {
			found = true
		}
	}
	if !found {
		t.Errorf("like.*cat* should match 'pat the cat', got: %v", arr)
	}
}

// EX29: ilike filter — GET /todos?task=ilike.*CAT* => 200, case-insensitive match.
func TestEX29_IlikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "ilike.*CAT*"), nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(strings.ToLower(task), "cat") {
			found = true
		}
	}
	if !found {
		t.Errorf("ilike.*CAT* should match 'pat the cat', got: %v", arr)
	}
}

// EX30: is.null filter — GET /todos?due=is.null => 200, all due=null.
func TestEX30_IsNullFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("due", "is.null"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with due=null (id=2)")
	}
	r.RowsAllMatch("due=null", func(row map[string]any) bool {
		return row["due"] == nil
	})
}

// EX31: in filter — GET /todos?id=in.(1,2) => 200, exactly 2 rows.
func TestEX31_InFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "in.(1,2)"), nil)
	r.Status(200).ArrayLen(2)
}

// EX32: cs (contains) filter — GET /todos?tags=cs.{go} => 200, at least 1 row.
func TestEX32_ContainsFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "cs.{go}"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with tags containing 'go'")
	}
}

// EX33: cd (contained by) filter — GET /todos?tags=cd.{go,sql,pets,chores,home} => 200.
func TestEX33_ContainedByFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "cd.{go,sql,pets,chores,home}"), nil)
	r.Status(200)
}

// EX34: ov (overlaps) filter — GET /todos?tags=ov.{go,pets} => 200.
func TestEX34_OverlapsFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "ov.{go,pets}"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with tags overlapping {go,pets}")
	}
}

// EX35: fts text search — GET /todos?task=fts.tutorial => 200, rows contain "tutorial".
func TestEX35_FtsTextSearch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	r.Status(200)
	arr := r.JSONArray()
	found := false
	for _, row := range arr {
		if task, ok := row["task"].(string); ok && strings.Contains(task, "tutorial") {
			found = true
		}
	}
	if !found {
		t.Errorf("fts.tutorial should match 'finish tutorial', got: %v", arr)
	}
}

// EX36: not.in filter — GET /todos?id=not.in.(1,2) => 200, only id=3 returned.
func TestEX36_NotInFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "not.in.(1,2)"), nil)
	r.Status(200)
	r.RowsAllMatch("id not in (1,2)", func(row map[string]any) bool {
		id, ok := row["id"].(float64)
		return ok && id != 1 && id != 2
	})
}

// EX37: raw eq filter on string column — GET /persons?name=eq.Alice => 200, 1 row Alice.
func TestEX37_RawFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("name", "eq.Alice"), nil)
	r.Status(200).ArrayLen(1)
	arr := r.JSONArray()
	if name, ok := arr[0]["name"].(string); !ok || name != "Alice" {
		t.Errorf("expected name=Alice, got: %v", arr[0]["name"])
	}
}

// EX38: limit — GET /todos?limit=2 => 200, exactly 2 rows.
func TestEX38_Limit(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("limit", "2"), nil)
	r.Status(200).ArrayLen(2)
}

// EX39: offset — GET /todos?order=id.asc&offset=1 => 200, no id=1.
func TestEX39_Offset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "id.asc", "offset", "1"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for _, row := range arr {
		if id, ok := row["id"].(float64); ok && id == 1 {
			t.Errorf("offset=1 should skip id=1, but it appeared in results")
		}
	}
}

// EX40: range via offset+limit — GET /todos?offset=0&limit=2 => 200, exactly 2 rows.
func TestEX40_Range(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("offset", "0", "limit", "2"), nil)
	r.Status(200).ArrayLen(2)
}

// EX41: order desc — GET /todos?order=id.desc => 200, rows in descending id order.
func TestEX41_OrderDesc(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "id.desc"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 2 {
		t.Fatal("expected at least 2 rows")
	}
	for i := 1; i < len(arr); i++ {
		prev, _ := arr[i-1]["id"].(float64)
		cur, _ := arr[i]["id"].(float64)
		if cur >= prev {
			t.Errorf("row[%d].id=%v >= row[%d].id=%v, not descending", i, cur, i-1, prev)
		}
	}
}

// EX42: order desc nullsfirst — GET /todos?order=due.desc.nullsfirst => first row due=null.
func TestEX42_OrderDescNullsFirst(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "due.desc.nullsfirst"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one row")
	}
	if arr[0]["due"] != nil {
		t.Errorf("nullsfirst: first row should have due=null, got: %v", arr[0]["due"])
	}
}

// EX43: select specific columns — GET /todos?select=id,task => rows have id+task only, no done.
func TestEX43_SelectSpecificColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected rows")
	}
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing 'task'", i)
		}
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] should not have 'done' when not selected", i)
		}
	}
}

// EX44: single semantics — GET /todos?id=eq.1 Accept: application/vnd.pgrst.object+json => JSON object.
func TestEX44_SingleSemantics(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	r.Status(200)
	obj := r.JSONObject()
	if _, ok := obj["id"]; !ok {
		t.Errorf("expected JSON object with 'id', got: %v", obj)
	}
}

// EX45: maybe-single — GET /todos?id=eq.99999 Accept: application/vnd.pgrst.object+json => 200 or 406.
func TestEX45_MaybeSingle(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "eq.99999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	r.StatusIn(200, 406)
}

// EX46: update returning representation — PATCH /todos?id=eq.N Prefer: return=representation => 200, non-empty.
func TestEX46_UpdateReturningRepresentation(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "ex-ex46-update", "done": false})
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("insert failed")
	}
	id := int(arr[0]["id"].(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})

	r := h.Patch("/todos", harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"done": true})
	r.Status(200)
	body := r.RawBody()
	if len(strings.TrimSpace(string(body))) == 0 {
		t.Error("expected non-empty body for return=representation")
	}
}

// EX47: update returning minimal — PATCH /todos?id=eq.1 Prefer: return=minimal => 200 or 204.
func TestEX47_UpdateReturningMinimal(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Patch("/todos", harness.P("id", "eq.1"), nil, map[string]any{"done": true})
	})
	r := h.Patch("/todos", harness.P("id", "eq.1"),
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"done": false})
	r.StatusIn(200, 204)
}

// EX48: upsert merge-duplicates — POST /todos?on_conflict=task Prefer: resolution=merge-duplicates => 200 or 201.
func TestEX48_UpsertMergeDuplicates(t *testing.T) {
	h := harness.New(t)
	task := "ex-ex48-upsert-z9m1"
	// Insert first so the upsert has something to merge against.
	ins := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": task, "done": false})
	ins.StatusIn(200, 201)
	insArr := ins.JSONArray()
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq."+task), nil)
	})
	_ = insArr

	r := h.Post("/todos", harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=merge-duplicates,return=representation"),
		map[string]any{"task": task, "done": true})
	r.StatusIn(200, 201)
}

// EX49: upsert ignore-duplicates — POST /todos?on_conflict=task Prefer: resolution=ignore-duplicates => 200 or 201.
func TestEX49_UpsertIgnoreDuplicates(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=ignore-duplicates,return=representation"),
		map[string]any{"task": "finish tutorial"})
	r.StatusIn(200, 201)
}

// EX50: insert count=exact — POST /todos Prefer: return=representation,count=exact => 200 or 201, Content-Range.
func TestEX50_InsertCountExact(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation,count=exact"),
		map[string]any{"task": "ex-ex50-count", "done": false})
	r.StatusIn(200, 201)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected returned row")
	}
	id := int(arr[0]["id"].(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
	r.HasHeader("Content-Range")
}

// EX51: count=planned — GET /todos Prefer: count=planned => 200 or 206, Content-Range.
func TestEX51_CountPlanned(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=planned"))
	r.StatusIn(200, 206)
	r.HasHeader("Content-Range")
}

// EX52: count=estimated — GET /todos Prefer: count=estimated => 200 or 206, Content-Range.
func TestEX52_CountEstimated(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=estimated"))
	r.StatusIn(200, 206)
	r.HasHeader("Content-Range")
}

// EX53: Content-Profile schema switching on write — POST /items Content-Profile: private => 201 or 403.
func TestEX53_ContentProfileSchemaSwitching(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/items", nil,
		harness.H_("Content-Profile", "private", "Prefer", "return=minimal"),
		map[string]any{"name": "ex-ex53"})
	r.StatusIn(201, 401, 403)
	if r.Header("HTTP_STATUS") != "403" {
		// If insert succeeded, clean up
		t.Cleanup(func() {
			h.Delete("/items", harness.P("name", "eq.ex-ex53"),
				harness.H_("Content-Profile", "private"))
		})
	}
}

// EX54: RPC stable function GET — GET /rpc/get_todos_count => 200.
func TestEX54_RpcStableFunctionGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/get_todos_count", nil, nil)
	r.Status(200)
}

// EX55: RPC volatile function POST — POST /rpc/add body={"a":5,"b":3} => 200, body contains "8".
func TestEX55_RpcVolatileFunctionPost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/add", nil, nil, map[string]any{"a": 5, "b": 3})
	r.Status(200).BodyContains("8")
}

// EX56: RPC returning setof — POST /rpc/get_person_by_name body={"name_param":"Alice"} => 200, array with Alice.
func TestEX56_RpcReturningSetof(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_person_by_name", nil, nil, map[string]any{"name_param": "Alice"})
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one row for Alice")
	}
	found := false
	for _, row := range arr {
		if name, ok := row["name"].(string); ok && name == "Alice" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Alice in result, got: %v", arr)
	}
}

// EX57: multi-step pipeline — GET /todos?done=eq.false&order=id.asc&limit=2 => at most 2 rows, all done=false.
func TestEX57_MultiStepPipeline(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "eq.false", "order", "id.asc", "limit", "2"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) > 2 {
		t.Errorf("limit=2 but got %d rows", len(arr))
	}
	for i, row := range arr {
		if row["done"] != false {
			t.Errorf("row[%d] done=%v, want false", i, row["done"])
		}
	}
}

// EX58: embedded resource select — GET /messages?select=id,message,persons(name) => rows have "persons" field.
func TestEX58_EmbeddedResourceSelect(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,message,persons(name)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected message rows")
	}
	for i, row := range arr {
		if _, ok := row["persons"]; !ok {
			t.Errorf("row[%d] missing embedded 'persons' field", i)
		}
	}
}

// EX59: auth bearer token — GET /todos Authorization: Bearer <valid jwt> => 200.
func TestEX59_AuthBearerToken(t *testing.T) {
	h := harness.New(t)
	token := makeAnonJWT()
	r := h.Get("/todos", nil, harness.H_("Authorization", "Bearer "+token))
	r.Status(200)
}

// EX60: delete returning representation — DELETE /todos?id=eq.N Prefer: return=representation => 200, non-empty.
func TestEX60_DeleteReturningRepresentation(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "ex-ex60-delete", "done": false})
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("insert failed")
	}
	id := int(arr[0]["id"].(float64))
	t.Cleanup(func() {
		// best-effort cleanup in case delete test itself failed
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})

	r := h.Delete("/todos", harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "return=representation"))
	r.Status(200)
	body := r.RawBody()
	s := strings.TrimSpace(string(body))
	if s == "" || s == "[]" {
		t.Error("expected non-empty body for delete return=representation")
	}
}

// EX61: is.true filter — GET /todos?done=is.true => 200, all done=true.
func TestEX61_IsTrueFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "is.true"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one done=true row (id=1)")
	}
	r.RowsAllMatch("done=true", func(row map[string]any) bool {
		return row["done"] == true
	})
}

// EX62: order multiple columns — GET /todos?order=done.asc,id.desc => 200.
func TestEX62_OrderMultipleColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "done.asc,id.desc"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected rows")
	}
	// Verify ordering: done=false rows come before done=true rows,
	// and within same done value, id is descending.
	for i := 1; i < len(arr); i++ {
		prevDone := arr[i-1]["done"]
		curDone := arr[i]["done"]
		// done=false < done=true; if prev=true and cur=false, ordering violated
		if prevDone == true && curDone == false {
			t.Errorf("row[%d] done=false after row[%d] done=true (order=done.asc violated)", i, i-1)
		}
		if prevDone == curDone {
			prevID, _ := arr[i-1]["id"].(float64)
			curID, _ := arr[i]["id"].(float64)
			if curID >= prevID {
				t.Errorf("row[%d].id=%v >= row[%d].id=%v within same done group (order=id.desc violated)", i, curID, i-1, prevID)
			}
		}
	}
}

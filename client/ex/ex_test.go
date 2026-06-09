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
// PostgREST returns GeoJSON for geometry columns; for non-geometry tables it may
// return 415. Accept both 200 and 415 as best-effort.
// Elixir: Q.with_custom_media_type(q, :postgis) => Accept: application/geo+json
func TestEX3_GeoJSONPreset(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Accept", "application/geo+json"))
	r.StatusIn(200, 406, 415)
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

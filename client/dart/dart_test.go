// Package dart_test contains wire compatibility tests for the Dart/Flutter
// postgrest client (pub.dev package "postgrest").
//
// These tests send the exact HTTP requests the Dart client would generate and
// verify that the server returns correct status codes, headers, and JSON bodies.
//
// Run against PostgREST:
//
//	go test ./client/dart/... -v
//
// Run against dbrest:
//
//	POSTGREST_URL=http://localhost:3001 go test ./client/dart/... -v
package dart_test

import (
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// DA1: FetchOptions count=exact => Prefer: count=exact, Content-Range with total
func TestDA1_CountExact(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Prefer", "count=exact"),
	)
	r.Status(200).HasHeader("Content-Range")
	cr := r.Header("Content-Range")
	if harness.ContentRangeTotal(cr) < 0 {
		t.Errorf("Content-Range total not present: %q", cr)
	}
}

// DA2: FetchOptions count=planned => Prefer: count=planned, Content-Range present
// count=planned uses EXPLAIN; 206 is expected when the estimate exceeds
// actual rows (e.g., fresh table with stale statistics before ANALYZE).
func TestDA2_CountPlanned(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		nil,
		harness.H_("Prefer", "count=planned"),
	)
	r.StatusIn(200, 206).HasHeader("Content-Range")
}

// DA3: FetchOptions count=estimated => Prefer: count=estimated, Content-Range present
// Same as DA2: accept both 200 and 206.
func TestDA3_CountEstimated(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		nil,
		harness.H_("Prefer", "count=estimated"),
	)
	r.StatusIn(200, 206).HasHeader("Content-Range")
}

// DA4: FetchOptions head=true => HEAD request, empty body, Content-Range present
func TestDA4_HeadRequest(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, nil)
	r.Status(200).EmptyBody().HasHeader("Content-Range")
}

// DA5: FK embed person via messages => GET /messages?select=id,message,persons(name)
func TestDA5_FKEmbedPerson(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages",
		harness.P("select", "id,message,persons(name)"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows, got empty array")
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing field 'id'", i)
		}
		if _, ok := row["message"]; !ok {
			t.Errorf("row[%d] missing field 'message'", i)
		}
		if _, ok := row["persons"]; !ok {
			t.Errorf("row[%d] missing embedded 'persons'", i)
		}
	}
}

// DA6: FK embed country via cities => GET /cities?select=id,name,countries(name)
func TestDA6_FKEmbedCountry(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/cities",
		harness.P("select", "id,name,countries(name)"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows, got empty array")
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing field 'id'", i)
		}
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing field 'name'", i)
		}
		if _, ok := row["countries"]; !ok {
			t.Errorf("row[%d] missing embedded 'countries'", i)
		}
	}
}

// DA7: Insert + select chained => POST /todos?select=id,task Prefer: return=representation => 201 with rows containing id
func TestDA7_InsertWithSelect(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "dart insert test", "done": false},
	)
	r.StatusIn(200, 201)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected returned rows")
	}
	row := rows[0]
	idVal, ok := row["id"]
	if !ok {
		t.Fatal("inserted row missing 'id'")
	}
	t.Cleanup(func() {
		id := int(idVal.(float64))
		h.Delete("/todos",
			harness.P("id", "eq."+itoa(id)),
			nil,
		)
	})
	if _, ok := row["task"]; !ok {
		t.Error("inserted row missing 'task'")
	}
}

// DA8: Upsert merge => POST /todos Prefer: resolution=merge-duplicates
func TestDA8_UpsertMerge(t *testing.T) {
	h := harness.New(t)
	// First insert to get a known id
	ir := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "dart upsert merge", "done": false},
	)
	ir.StatusIn(200, 201)
	rows := ir.JSONArray()
	if len(rows) == 0 {
		t.Fatal("insert returned no rows")
	}
	id := int(rows[0]["id"].(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})

	// Now upsert with merge
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "resolution=merge-duplicates"),
		map[string]any{"id": id, "task": "dart upsert merge updated", "done": true},
	)
	r.StatusIn(200, 201)
}

// DA9: Upsert ignore => POST /todos Prefer: resolution=ignore-duplicates
func TestDA9_UpsertIgnore(t *testing.T) {
	h := harness.New(t)
	// First insert to get a known id
	ir := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "dart upsert ignore", "done": false},
	)
	ir.StatusIn(200, 201)
	rows := ir.JSONArray()
	if len(rows) == 0 {
		t.Fatal("insert returned no rows")
	}
	id := int(rows[0]["id"].(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})

	// Now upsert with ignore
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "resolution=ignore-duplicates"),
		map[string]any{"id": id, "task": "dart upsert ignore (should stay)", "done": false},
	)
	r.StatusIn(200, 201)
}

// DA10: UpsertOptions on_conflict => POST /todos?on_conflict=task Prefer: resolution=merge-duplicates
func TestDA10_UpsertOnConflict(t *testing.T) {
	h := harness.New(t)
	task := "dart on_conflict unique task"

	// Insert first
	ir := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": task, "done": false},
	)
	ir.StatusIn(200, 201)
	rows := ir.JSONArray()
	if len(rows) == 0 {
		t.Fatal("insert returned no rows")
	}
	id := int(rows[0]["id"].(float64))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})

	// Upsert on conflict by task column
	r := h.Post("/todos",
		harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=merge-duplicates"),
		map[string]any{"task": task, "done": true},
	)
	r.StatusIn(200, 201)
}

// DA11: Filter eq => GET /todos?done=eq.true => only done=true rows
func TestDA11_FilterEq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("done", "eq.true"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with done=true")
	}
	for i, row := range rows {
		if v, ok := row["done"].(bool); !ok || !v {
			t.Errorf("row[%d] done is not true: %v", i, row["done"])
		}
	}
}

// DA12: Filter neq => GET /todos?done=neq.true => done=false rows
func TestDA12_FilterNeq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("done", "neq.true"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with done=false")
	}
	for i, row := range rows {
		if v, ok := row["done"].(bool); !ok || v {
			t.Errorf("row[%d] done is not false: %v", i, row["done"])
		}
	}
}

// DA13: Filter like => GET /todos?task=like.*tutorial* => rows with tutorial in task
func TestDA13_FilterLike(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("task", "like.*tutorial*"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows matching like.*tutorial*")
	}
	for i, row := range rows {
		task, _ := row["task"].(string)
		if !strings.Contains(strings.ToLower(task), "tutorial") {
			t.Errorf("row[%d] task %q does not contain 'tutorial'", i, task)
		}
	}
}

// DA14: Filter in => GET /todos?id=in.(1,2,3) => 3 rows
func TestDA14_FilterIn(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "in.(1,2,3)"),
		nil,
	)
	r.Status(200).ArrayLen(3)
}

// DA15: Filter is null => GET /todos?due=is.null => rows with no due date
func TestDA15_FilterIsNull(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("due", "is.null"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row with due=null")
	}
	for i, row := range rows {
		if row["due"] != nil {
			t.Errorf("row[%d] due is not null: %v", i, row["due"])
		}
	}
}

// DA16: Transform order => GET /todos?select=*&order=id.desc => 3 rows in desc id order
func TestDA16_TransformOrder(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*", "order", "id.desc"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) < 2 {
		t.Fatal("expected multiple rows for ordering test")
	}
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]["id"].(float64)
		curr := rows[i]["id"].(float64)
		if prev < curr {
			t.Errorf("rows not in descending id order at index %d: %v > %v", i, prev, curr)
		}
	}
}

// DA17: Transform limit => GET /todos?select=*&limit=2 => 2 rows
func TestDA17_TransformLimit(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*", "limit", "2"),
		nil,
	)
	r.Status(200).ArrayLen(2)
}

// DA18: Transform range => GET /todos?select=*&offset=1&limit=2 => rows 2,3
// Dart client uses ?offset=from&limit=(to-from+1) (NOT Range header)
func TestDA18_TransformRange(t *testing.T) {
	h := harness.New(t)
	// range(1, 2) => offset=1, limit=2
	r := h.Get("/todos",
		harness.P("select", "*", "offset", "1", "limit", "2"),
		nil,
	)
	r.Status(200).ArrayLen(2)
}

// DA19: Single row => GET /todos?id=eq.1 Accept: application/vnd.pgrst.object+json => 200, single object
func TestDA19_SingleRow(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(200)
	obj := r.JSONObject()
	if _, ok := obj["id"]; !ok {
		t.Error("single object missing 'id'")
	}
}

// DA20: Single miss => GET /todos?id=eq.9999 Accept: application/vnd.pgrst.object+json => 406
func TestDA20_SingleMiss(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.9999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406)
}

// DA21: MaybeSingle miss => GET /todos?id=eq.9999 Accept: application/json => 200, []
func TestDA21_MaybeSingleMiss(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.9999"),
		harness.H_("Accept", "application/json"),
	)
	r.Status(200).ArrayLen(0)
}

// DA22: CSV => GET /todos?select=id,task Accept: text/csv => 200, Content-Type: text/csv
func TestDA22_CSV(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "id,task"),
		harness.H_("Accept", "text/csv"),
	)
	r.Status(200).ContentType("text/csv")
	body := string(r.RawBody())
	if !strings.Contains(body, "id") || !strings.Contains(body, "task") {
		t.Errorf("CSV body missing expected headers, got: %s", body)
	}
}

// DA23: PGRST error => GET /nonexistent => 404, error envelope with code field
func TestDA23_PGRSTError(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/nonexistent", nil, nil)
	r.Status(404)
	// Check the error envelope has a code field (PostgrestException format)
	obj := r.JSONObject()
	if _, ok := obj["code"]; !ok {
		t.Error("error envelope missing 'code' field (PostgrestException requires code/message)")
	}
}

// itoa converts an int to a string without importing strconv at the top level.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

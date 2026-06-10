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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
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

// makeAnonymousJWT builds a minimal HS256 JWT with role=web_anon.
func makeAnonymousJWT() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"web_anon"}`))
	msg := header + "." + payload
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// insertTodo inserts a todo with the given task name and returns its id.
// The caller must register a t.Cleanup to delete the row.
func insertTodo(h *harness.H, task string) int {
	r := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": task, "done": false},
	)
	r.StatusIn(200, 201)
	rows := r.JSONArray()
	if len(rows) == 0 {
		panic("insertTodo: no rows returned for task " + task)
	}
	return int(rows[0]["id"].(float64))
}

// DA24: GET /todos?select=* => 200, 3 rows
func TestDA24_SelectAll(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "*"), nil)
	r.Status(200).ArrayLen(3)
}

// DA25: GET /todos?select=id,task => 200, rows have id+task, no done/due
func TestDA25_SelectColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing 'task'", i)
		}
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] unexpected field 'done'", i)
		}
		if _, ok := row["due"]; ok {
			t.Errorf("row[%d] unexpected field 'due'", i)
		}
	}
}

// DA26: GET /persons?age=gt.29 => 200, all age>29
func TestDA26_FilterGt(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("age", "gt.29"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with age>29")
	}
	for i, row := range rows {
		age := row["age"].(float64)
		if age <= 29 {
			t.Errorf("row[%d] age %v is not >29", i, age)
		}
	}
}

// DA27: GET /persons?age=gte.30 => 200, all age>=30
func TestDA27_FilterGte(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("age", "gte.30"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with age>=30")
	}
	for i, row := range rows {
		age := row["age"].(float64)
		if age < 30 {
			t.Errorf("row[%d] age %v is not >=30", i, age)
		}
	}
}

// DA28: GET /persons?age=lt.30 => 200, all age<30
func TestDA28_FilterLt(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("age", "lt.30"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with age<30")
	}
	for i, row := range rows {
		age := row["age"].(float64)
		if age >= 30 {
			t.Errorf("row[%d] age %v is not <30", i, age)
		}
	}
}

// DA29: GET /persons?age=lte.30 => 200, all age<=30
func TestDA29_FilterLte(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("age", "lte.30"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows with age<=30")
	}
	for i, row := range rows {
		age := row["age"].(float64)
		if age > 30 {
			t.Errorf("row[%d] age %v is not <=30", i, age)
		}
	}
}

// DA30: GET /todos?task=ilike.*TUTORIAL* => 200, rows contain "tutorial" (case-insensitive)
func TestDA30_FilterIlike(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "ilike.*TUTORIAL*"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows matching ilike.*TUTORIAL*")
	}
	for i, row := range rows {
		task, _ := row["task"].(string)
		if !strings.Contains(strings.ToLower(task), "tutorial") {
			t.Errorf("row[%d] task %q does not contain 'tutorial'", i, task)
		}
	}
}

// DA31: GET /todos?done=is.true => 200, all done=true
func TestDA31_FilterIsTrue(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "is.true"), nil)
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

// DA32: GET /todos?done=is.false => 200, all done=false
func TestDA32_FilterIsFalse(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "is.false"), nil)
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

// DA33: GET /todos?tags=cs.{go} => 200, at least 1 row (id=1 has go in tags)
func TestDA33_FilterContains(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "cs.{go}"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row whose tags contain 'go'")
	}
}

// DA34: GET /todos?tags=cd.{go,sql,pets,chores,home} => 200
func TestDA34_FilterContainedBy(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "cd.{go,sql,pets,chores,home}"), nil)
	r.Status(200)
}

// DA35: GET /todos?tags=ov.{go,rust} => 200 (may be 0 rows, just verify no error)
func TestDA35_FilterOverlaps(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "ov.{go,rust}"), nil)
	r.Status(200)
}

// DA36: GET /todos?task=fts.tutorial => 200
func TestDA36_FilterFts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	r.Status(200)
}

// DA37: GET /todos?task=plfts.tutorial => 200
func TestDA37_FilterPlfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "plfts.tutorial"), nil)
	r.Status(200)
}

// DA38: GET /todos?task=phfts.finish tutorial => 200
func TestDA38_FilterPhfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "phfts.finish tutorial"), nil)
	r.Status(200)
}

// DA39: GET /todos?task=wfts.tutorial => 200
func TestDA39_FilterWfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "wfts.tutorial"), nil)
	r.Status(200)
}

// DA40: GET /todos?done=not.eq.true => 200, all done=false
func TestDA40_FilterNotEq(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "not.eq.true"), nil)
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

// DA41: GET /todos?id=not.in.(1,2) => 200, no id=1 or id=2
func TestDA41_FilterNotIn(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "not.in.(1,2)"), nil)
	r.Status(200)
	for i, row := range r.JSONArray() {
		id := row["id"].(float64)
		if id == 1 || id == 2 {
			t.Errorf("row[%d] id %v should have been excluded", i, id)
		}
	}
}

// DA42: GET /todos?or=(id.eq.1,id.eq.2) => 200, exactly 2 rows
func TestDA42_FilterOr(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("or", "(id.eq.1,id.eq.2)")
	r := h.Get("/todos", params, nil)
	r.Status(200).ArrayLen(2)
}

// DA43: GET /todos?and=(done.eq.false,id.eq.3) => 200, 1 row (id=3, done=false)
func TestDA43_FilterAnd(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("and", "(done.eq.false,id.eq.3)")
	r := h.Get("/todos", params, nil)
	r.Status(200).ArrayLen(1)
}

// DA44: GET /todos?done=eq.false&id=eq.3 => 200, 1 row
func TestDA44_FilterMatch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "eq.false", "id", "eq.3"), nil)
	r.Status(200).ArrayLen(1)
}

// DA45: GET /persons?name=eq.Alice => 200, 1 row
func TestDA45_FilterRaw(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/persons", harness.P("name", "eq.Alice"), nil)
	r.Status(200).ArrayLen(1)
}

// DA46: GET /todos?select=id&order=id.asc => 200, ascending id
func TestDA46_OrderAsc(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id", "order", "id.asc"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) < 2 {
		t.Fatal("expected multiple rows")
	}
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]["id"].(float64)
		curr := rows[i]["id"].(float64)
		if prev > curr {
			t.Errorf("rows not in ascending id order at index %d: %v > %v", i, prev, curr)
		}
	}
}

// DA47: GET /todos?select=id,due&order=due.desc.nullsfirst => 200, first row has due=null
func TestDA47_OrderNullsFirst(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,due", "order", "due.desc.nullsfirst"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	if rows[0]["due"] != nil {
		t.Errorf("first row should have due=null (nullsfirst), got: %v", rows[0]["due"])
	}
}

// DA48: GET /todos?select=id,due&order=due.asc.nullslast => 200, last row has due=null
func TestDA48_OrderNullsLast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,due", "order", "due.asc.nullslast"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	if rows[len(rows)-1]["due"] != nil {
		t.Errorf("last row should have due=null (nullslast), got: %v", rows[len(rows)-1]["due"])
	}
}

// DA49: GET /channels?select=id,messages(id,message)&messages.order=id.asc => 200
func TestDA49_OrderEmbedded(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels",
		harness.P("select", "id,messages(id,message)", "messages.order", "id.asc"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA50: GET /channels?select=id,messages(id,message)&messages.limit=1 => 200
func TestDA50_LimitEmbedded(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels",
		harness.P("select", "id,messages(id,message)", "messages.limit", "1"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA51: GET /channels?select=id,messages(id,message)&messages.offset=0&messages.limit=2 => 200
func TestDA51_RangeEmbedded(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels",
		harness.P("select", "id,messages(id,message)", "messages.offset", "0", "messages.limit", "2"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA52: POST /todos body={"task":"dart-da52-noreturn","done":false} no Prefer return => 201
func TestDA52_InsertNoReturn(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil, nil,
		map[string]any{"task": "dart-da52-noreturn", "done": false},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.dart-da52-noreturn"), nil)
	})
}

// DA53: POST /todos body=[{...},{...}] => 201
func TestDA53_InsertBulk(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil, nil,
		[]map[string]any{
			{"task": "dart-da53-bulk-a", "done": false},
			{"task": "dart-da53-bulk-b", "done": false},
		},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos",
			harness.P("task", "in.(dart-da53-bulk-a,dart-da53-bulk-b)"),
			nil,
		)
	})
}

// DA54: POST /todos Prefer: missing=default body={"task":"dart-da54-missingdefault"} => 201
func TestDA54_InsertMissingDefault(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "missing=default"),
		map[string]any{"task": "dart-da54-missingdefault"},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.dart-da54-missingdefault"), nil)
	})
}

// DA55: POST /todos Prefer: return=minimal => 201
func TestDA55_InsertReturnMinimal(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"task": "dart-da55-minimal", "done": false},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.dart-da55-minimal"), nil)
	})
}

// DA56: POST /todos Prefer: return=headers-only => 201, has Location header
func TestDA56_InsertReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "return=headers-only"),
		map[string]any{"task": "dart-da56-headersonly", "done": false},
	)
	r.Status(201).HasHeader("Location")
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.dart-da56-headersonly"), nil)
	})
}

// DA57: POST /todos?columns=task,done Prefer: return=representation body=[{...}] => 201
func TestDA57_InsertColumns(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("columns", "task,done")
	r := h.Post("/todos", params,
		harness.H_("Prefer", "return=representation"),
		[]map[string]any{{"task": "dart-da57-cols", "done": false}},
	)
	r.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.dart-da57-cols"), nil)
	})
}

// DA58: insert row then PATCH /todos?id=eq.X body={"done":true} => StatusIn(200,204)
func TestDA58_UpdateNoReturn(t *testing.T) {
	h := harness.New(t)
	id := insertTodo(h, "dart-da58-update")
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
	r := h.Patch("/todos", harness.P("id", "eq."+itoa(id)), nil,
		map[string]any{"done": true},
	)
	r.StatusIn(200, 204)
}

// DA59: insert row then PATCH /todos?id=eq.X&select=id,task Prefer: return=representation => 200, rows non-empty
func TestDA59_UpdateReturn(t *testing.T) {
	h := harness.New(t)
	id := insertTodo(h, "dart-da59-updatereturn")
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
	r := h.Patch("/todos",
		harness.P("id", "eq."+itoa(id), "select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"done": false},
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected non-empty returned rows")
	}
}

// DA60: insert row then PATCH /todos?id=eq.X Prefer: count=exact => StatusIn(200,204), has Content-Range
func TestDA60_UpdateCount(t *testing.T) {
	h := harness.New(t)
	id := insertTodo(h, "dart-da60-updatecount")
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
	r := h.Patch("/todos",
		harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "count=exact"),
		map[string]any{"done": true},
	)
	r.StatusIn(200, 204).HasHeader("Content-Range")
}

// DA61: PATCH /todos Prefer: return=minimal body={"done":false} => StatusIn(200,204,400)
func TestDA61_UpdateNoFilter(t *testing.T) {
	h := harness.New(t)
	// Restore seed done values in cleanup so later packages see consistent state.
	t.Cleanup(func() {
		h.Patch("/todos", harness.P("id", "eq.1"), nil, map[string]any{"done": true})
		h.Patch("/todos", harness.P("id", "eq.2"), nil, map[string]any{"done": true})
	})
	r := h.Patch("/todos", nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"done": false},
	)
	r.StatusIn(200, 204, 400)
}

// DA62: insert row, DELETE /todos?task=eq.dart-da62-delete => StatusIn(200,204)
func TestDA62_DeleteFilter(t *testing.T) {
	h := harness.New(t)
	insertTodo(h, "dart-da62-delete")
	r := h.Delete("/todos", harness.P("task", "eq.dart-da62-delete"), nil)
	r.StatusIn(200, 204)
}

// DA63: insert row, DELETE /todos?task=eq.X&select=id,task Prefer: return=representation => 200, non-empty body
func TestDA63_DeleteReturn(t *testing.T) {
	h := harness.New(t)
	insertTodo(h, "dart-da63-deletereturn")
	r := h.Delete("/todos",
		harness.P("task", "eq.dart-da63-deletereturn", "select", "id,task"),
		harness.H_("Prefer", "return=representation"),
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected non-empty returned rows")
	}
}

// DA64: insert row, DELETE /todos?task=eq.X Prefer: count=exact => StatusIn(200,204), has Content-Range
func TestDA64_DeleteCount(t *testing.T) {
	h := harness.New(t)
	insertTodo(h, "dart-da64-deletecount")
	r := h.Delete("/todos",
		harness.P("task", "eq.dart-da64-deletecount"),
		harness.H_("Prefer", "count=exact"),
	)
	r.StatusIn(200, 204).HasHeader("Content-Range")
}

// DA65: POST /rpc/add body={"a":1,"b":2} => 200, body contains "3"
func TestDA65_RpcPost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/add", nil, nil, map[string]any{"a": 1, "b": 2})
	r.Status(200).BodyContains("3")
}

// DA66: GET /rpc/add?a=1&b=2 => 200, body contains "3"
func TestDA66_RpcGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/add", harness.P("a", "1", "b", "2"), nil)
	r.Status(200).BodyContains("3")
}

// DA67: HEAD /rpc/get_todos_count => 200, empty body
func TestDA67_RpcHead(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/rpc/get_todos_count", nil, nil)
	r.Status(200).EmptyBody()
}

// DA68: POST /rpc/get_todos_count Prefer: count=exact body={} => 200
func TestDA68_RpcCount(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_todos_count", nil,
		harness.H_("Prefer", "count=exact"),
		map[string]any{},
	)
	r.Status(200)
}

// DA69: GET /rpc/get_person_by_name?name_param=Alice Accept: application/vnd.pgrst.object+json => StatusIn(200,406)
func TestDA69_RpcSingle(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/get_person_by_name",
		harness.P("name_param", "Alice"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.StatusIn(200, 406)
}

// DA70: POST /rpc/get_person_by_name body={"name_param":"Alice"} => 200, array with Alice
func TestDA70_RpcReturnsRows(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_person_by_name", nil, nil, map[string]any{"name_param": "Alice"})
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows returned for name_param=Alice")
	}
	found := false
	for _, row := range rows {
		if name, _ := row["name"].(string); name == "Alice" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Alice not found in result: %v", rows)
	}
}

// DA71: GET /todos Authorization: Bearer <valid jwt> => 200
func TestDA71_JWTAuth(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil,
		harness.H_("Authorization", "Bearer "+makeAnonymousJWT()),
	)
	r.Status(200)
}

// DA72: GET /items Accept-Profile: private => 200, at least 1 row
func TestDA72_SchemaGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/items", nil,
		harness.H_("Accept-Profile", "private"),
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row in private.items")
	}
}

// DA73: POST /items Content-Profile: private body={"name":"dart-da73"} => StatusIn(201,403)
func TestDA73_SchemaPost(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/items", nil,
		harness.H_("Content-Profile", "private"),
		map[string]any{"name": "dart-da73"},
	)
	r.StatusIn(201, 401, 403)
}

// DA74: HEAD /todos Prefer: count=exact => 200, empty body, has Content-Range with numeric total
func TestDA74_HeadWithCount(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200).EmptyBody().HasHeader("Content-Range")
	cr := r.Header("Content-Range")
	if total := harness.ContentRangeTotal(cr); total < 0 {
		t.Errorf("Content-Range total not present or is *: %q", cr)
	}
}

// DA75: POST /todos Prefer: tx=rollback,return=representation => StatusIn(200,201); then GET => 0 rows
func TestDA75_TxRollback(t *testing.T) {
	const task = "dart-da75-txrollback-x7k9"
	h := harness.New(t)
	r := h.Post("/todos", nil,
		harness.H_("Prefer", "tx=rollback,return=representation"),
		map[string]any{"task": task, "done": false},
	)
	r.StatusIn(200, 201)
	// Verify the row was rolled back
	r2 := h.Get("/todos", harness.P("task", "eq."+task), nil)
	r2.Status(200).ArrayLen(0)
}

// DA76: GET /todos Accept: application/vnd.pgrst.object+json (no filter, multiple rows) => 406
func TestDA76_SingleManyRows406(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil,
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406)
}

// DA77: GET /todos?id=eq.9999 Accept: application/vnd.pgrst.object+json => 406; JSON has "code" field
func TestDA77_ErrorCodePGRST116(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.9999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406)
	obj := r.JSONObject()
	if _, ok := obj["code"]; !ok {
		t.Error("error envelope missing 'code' field")
	}
}

// DA78: GET /todos?select=nonexistent_col_xyz => 400, JSON has "code" field
func TestDA78_ErrorColumnNotFound(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "nonexistent_col_xyz"), nil)
	r.Status(400)
	obj := r.JSONObject()
	if _, ok := obj["code"]; !ok {
		t.Error("error envelope missing 'code' field")
	}
}

// DA79: GET /nonexistent_table_xyz => 404, JSON has "code" field
func TestDA79_ErrorTableNotFound(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/nonexistent_table_xyz", nil, nil)
	r.Status(404)
	obj := r.JSONObject()
	if _, ok := obj["code"]; !ok {
		t.Error("error envelope missing 'code' field")
	}
}

// DA80: GET /todos?select=task_text:task,id => 200, rows have "task_text" key
func TestDA80_SelectAlias(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "task_text:task,id"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	for i, row := range rows {
		if _, ok := row["task_text"]; !ok {
			t.Errorf("row[%d] missing alias 'task_text'", i)
		}
		if _, ok := row["task"]; ok {
			t.Errorf("row[%d] should not have original key 'task' when aliased", i)
		}
	}
}

// DA81: GET /messages?select=id,persons!inner(name) => 200
func TestDA81_EmbedInnerJoin(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages",
		harness.P("select", "id,persons!inner(name)"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA82: GET /channels?select=id,messages(id,message)&messages.channel_id=eq.1 => 200
func TestDA82_EmbedFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels",
		harness.P("select", "id,messages(id,message)", "messages.channel_id", "eq.1"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA83: GET /todos?select=*&order=done.asc,id.desc => 200
func TestDA83_OrderMultiCol(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*", "order", "done.asc,id.desc"),
		nil,
	)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA84: POST /todos Prefer: resolution=merge-duplicates,missing=default => StatusIn(200,201); cleanup
func TestDA84_UpsertMissingDefault(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "resolution=merge-duplicates,missing=default,return=representation"),
		map[string]any{"task": "dart-da84-upsertmissing", "done": false},
	)
	r.StatusIn(200, 201)
	rows := r.JSONArray()
	if len(rows) > 0 {
		id := int(rows[0]["id"].(float64))
		t.Cleanup(func() {
			h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
		})
	} else {
		t.Cleanup(func() {
			h.Delete("/todos", harness.P("task", "eq.dart-da84-upsertmissing"), nil)
		})
	}
}

// DA85: PATCH /todos?id=eq.1 Prefer: return=minimal body={"done":false} => StatusIn(200,204)
func TestDA85_UpdateReturnMinimal(t *testing.T) {
	h := harness.New(t)
	r := h.Patch("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"done": false},
	)
	r.StatusIn(200, 204)
}

// DA86: GET /todos?task=ilike(all).{*a*,*o*} => StatusIn(200,400)
func TestDA86_FilterIlikeAllOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "ilike(all).{*a*,*o*}"), nil)
	r.StatusIn(200, 400)
}

// DA87: GET /todos?task=ilike(any).{*CAT*,*LAUNDRY*} => StatusIn(200,400)
func TestDA87_FilterIlikeAnyOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "ilike(any).{*CAT*,*LAUNDRY*}"), nil)
	r.StatusIn(200, 400)
}

// DA88: GET /todos?task=like(all).{*a*,*o*} => StatusIn(200,400)
func TestDA88_FilterLikeAllOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "like(all).{*a*,*o*}"), nil)
	r.StatusIn(200, 400)
}

// DA89: GET /todos?task=like(any).{*cat*,*laundry*} => StatusIn(200,400)
func TestDA89_FilterLikeAnyOf(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "like(any).{*cat*,*laundry*}"), nil)
	r.StatusIn(200, 400)
}

// DA90: PATCH /todos?id=eq.1 Prefer: handling=strict,max-affected=1 body={"done":false} => StatusIn(200,204)
func TestDA90_MaxAffected(t *testing.T) {
	h := harness.New(t)
	r := h.Patch("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Prefer", "handling=strict,max-affected=1"),
		map[string]any{"done": false},
	)
	r.StatusIn(200, 204)
}

// DA91: HEAD /todos Prefer: count=exact => 200, has Content-Range with numeric total
func TestDA91_HeadCountExact(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200).EmptyBody().HasHeader("Content-Range")
	cr := r.Header("Content-Range")
	if total := harness.ContentRangeTotal(cr); total < 0 {
		t.Errorf("Content-Range total not numeric: %q", cr)
	}
}

// DA92: GET /messages?select=id,...persons(name) => StatusIn(200,400)
func TestDA92_EmbedSpread(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,...persons(name)"), nil)
	r.StatusIn(200, 400)
}

// DA93: GET /channels?select=id,messages(count) => 200
func TestDA93_EmbedCount(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels", harness.P("select", "id,messages(count)"), nil)
	r.Status(200)
	rows := r.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
}

// DA94: GET /rpc/get_todos_count Accept: application/vnd.pgrst.object+json => StatusIn(200,406)
func TestDA94_RpcGetSingle(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/get_todos_count", nil,
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.StatusIn(200, 406)
}

// DA95: insert then DELETE /todos?select=id Prefer: return=representation,count=exact => StatusIn(200,204)
func TestDA95_DeleteReturnAndCount(t *testing.T) {
	h := harness.New(t)
	insertTodo(h, "dart-da95-deletereturncount")
	r := h.Delete("/todos",
		harness.P("task", "eq.dart-da95-deletereturncount", "select", "id"),
		harness.H_("Prefer", "return=representation,count=exact"),
	)
	r.StatusIn(200, 204)
}

// DA96: GET /todos?id=eq.1 Accept: application/vnd.pgrst.object+json => 200, object (not array)
func TestDA96_MaybeSingleHit(t *testing.T) {
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

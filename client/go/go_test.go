// Package go_test verifies that the postgrest-go v0.0.12 client generates
// the correct HTTP requests and that the server returns the expected
// responses for each operation.
//
// Run against PostgREST:
//
//	go test ./client/go/... -v
//
// Run against dbrest:
//
//	POSTGREST_URL=http://localhost:3001 go test ./client/go/... -v
package go_test

import (
	"encoding/json"
	"fmt"
	"testing"

	postgrest "github.com/supabase-community/postgrest-go"

	"github.com/tamnd/postgrest-compat/harness"
)

// newClient returns a postgrest-go client pointed at the test server.
// Schema "api" maps to the root path (no /api/ prefix in URLs).
func newClient() *postgrest.Client {
	return postgrest.NewClient(harness.ServerURL(), "api", nil)
}

// ---------------------------------------------------------------------------
// SELECT tests
// ---------------------------------------------------------------------------

func TestSelectAll(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) < 3 {
		t.Errorf("expected at least 3 todos, got %d", len(rows))
	}
}

func TestSelectColumns(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("id,task", "", false).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing 'task'", i)
		}
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] unexpected 'done'", i)
		}
	}
}

func TestSelectWithCount(t *testing.T) {
	h := harness.New(t)
	// Use harness to verify Content-Range header with exact count
	h.Get("/todos", harness.P("select", "*"), harness.H_("Prefer", "count=exact")).
		Status(200).
		HasHeader("Content-Range")
}

func TestSelectHead(t *testing.T) {
	// HEAD request via harness
	h := harness.New(t)
	h.Head("/todos", harness.P("select", "*"), nil).
		Status(200).
		EmptyBody()
}

// ---------------------------------------------------------------------------
// FILTER operator tests
// ---------------------------------------------------------------------------

func TestFilterEq(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Eq("name", "Alice").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", rows[0]["name"])
	}
}

func TestFilterNeq(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Neq("name", "Alice").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		if row["name"] == "Alice" {
			t.Errorf("Alice should be excluded")
		}
	}
}

func TestFilterGt(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Gt("age", "25").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		age, _ := row["age"].(float64)
		if age <= 25 {
			t.Errorf("expected age > 25, got %v", age)
		}
	}
}

func TestFilterGte(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Gte("age", "30").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		age, _ := row["age"].(float64)
		if age < 30 {
			t.Errorf("expected age >= 30, got %v", age)
		}
	}
}

func TestFilterLt(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Lt("age", "30").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		age, _ := row["age"].(float64)
		if age >= 30 {
			t.Errorf("expected age < 30, got %v", age)
		}
	}
}

func TestFilterLte(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Lte("age", "30").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		age, _ := row["age"].(float64)
		if age > 30 {
			t.Errorf("expected age <= 30, got %v", age)
		}
	}
}

func TestFilterLike(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Like("name", "Ali%").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) == 0 {
		t.Error("expected at least one row with name like 'Ali%'")
	}
	for _, row := range rows {
		name, _ := row["name"].(string)
		if len(name) < 3 || name[:3] != "Ali" {
			t.Errorf("unexpected row name: %v", name)
		}
	}
}

func TestFilterIlike(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Ilike("name", "alice").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) == 0 {
		t.Error("expected ilike to match Alice case-insensitively")
	}
}

func TestFilterIs(t *testing.T) {
	// persons with no email (null) — Carol's email should be present so check done field on todos
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).Is("due", "null").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		if row["due"] != nil {
			t.Errorf("expected due=null, got %v", row["due"])
		}
	}
}

func TestFilterIn(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).In("name", []string{"Alice", "Bob"}).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows (Alice+Bob), got %d", len(rows))
	}
}

func TestFilterInSpecialChars(t *testing.T) {
	// Verify that values with commas are double-quoted by the client.
	// We use the harness to send the exact URL and check the server handles it.
	h := harness.New(t)
	// The client would send: ?name=in.("val,with,comma") — no such name exists
	h.Get("/persons", harness.P("name", `in.("val,with,comma")`), nil).
		Status(200).
		ArrayLen(0)
}

func TestFilterContains(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).Contains("tags", []string{"go"}).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	// todo id=1 has tags {go,sql}
	if len(rows) == 0 {
		t.Error("expected rows with tags containing 'go'")
	}
}

func TestFilterContainedBy(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).ContainedBy("tags", []string{"go", "sql", "pets", "chores", "home"}).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	// All todos should have tags that are subsets of the above
	if len(rows) == 0 {
		t.Error("expected some rows where tags are contained by the given set")
	}
}

func TestFilterOverlaps(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).Overlaps("tags", []string{"go"}).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) == 0 {
		t.Error("expected rows overlapping tags {go}")
	}
}

func TestFilterNot(t *testing.T) {
	// postgrest-go Not() internally calls Filter() with "not.eq" which fails
	// the library's own operator validation (library bug v0.0.12). Use Filter
	// with "neq" (semantically identical on the wire) as the workaround.
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Filter("name", "neq", "Alice").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for _, row := range rows {
		if row["name"] == "Alice" {
			t.Error("Alice should be excluded by neq filter")
		}
	}
}

func TestFilterOr(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Or("name.eq.Alice,name.eq.Bob", "").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows (Alice+Bob via or), got %d", len(rows))
	}
}

func TestFilterMatch(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Match(map[string]string{"name": "Alice"}).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Alice" {
		t.Errorf("expected exactly Alice, got %v", rows)
	}
}

func TestFilterTextSearch(t *testing.T) {
	// Plain text search on task column
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).TextSearch("task", "tutorial", "english", "plain").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	// todo id=1 has task "finish tutorial"
	found := false
	for _, row := range rows {
		if row["task"] == "finish tutorial" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'finish tutorial' via plain text search")
	}
}

func TestFilterTextSearchWebsearch(t *testing.T) {
	h := harness.New(t)
	// wfts(english).tutorial => ?task=wfts(english).tutorial
	h.Get("/todos", harness.P("task", "wfts(english).tutorial"), nil).
		Status(200)
}

func TestFilterTextSearchPhrase(t *testing.T) {
	h := harness.New(t)
	// phfts.laundry
	h.Get("/todos", harness.P("task", "phfts.laundry"), nil).
		Status(200)
}

// ---------------------------------------------------------------------------
// appendFilter: double-filter on same column merges to ?and=
// ---------------------------------------------------------------------------

func TestAppendFilterMerge(t *testing.T) {
	// When two filters on the same column are added, the client merges them into
	// ?and=(col.op1.v1,col.op2.v2). The server must handle this and return the
	// correct (AND) result.
	h := harness.New(t)
	// No person has age eq 30 AND age eq 25 simultaneously
	h.Get("/persons", harness.P("and", "(age.eq.30,age.eq.25)"), nil).
		Status(200).
		ArrayLen(0)
}

func TestAppendFilterMergeMatchingRows(t *testing.T) {
	// Filter: name neq Carol AND age gte 25 — should return Alice (30) and Bob (25)
	h := harness.New(t)
	h.Get("/persons", harness.P("and", "(name.neq.Carol,age.gte.25)"), nil).
		Status(200)
}

// ---------------------------------------------------------------------------
// ORDER, LIMIT, RANGE
// ---------------------------------------------------------------------------

func TestOrderAscending(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).
		Order("age", &postgrest.OrderOpts{Ascending: true}).
		ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for i := 1; i < len(rows); i++ {
		a, _ := rows[i-1]["age"].(float64)
		b, _ := rows[i]["age"].(float64)
		if a > b {
			t.Errorf("rows not ascending at %d: %v > %v", i, a, b)
		}
	}
}

func TestOrderDescending(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).
		Order("age", &postgrest.OrderOpts{Ascending: false}).
		ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	for i := 1; i < len(rows); i++ {
		a, _ := rows[i-1]["age"].(float64)
		b, _ := rows[i]["age"].(float64)
		if a < b {
			t.Errorf("rows not descending at %d: %v < %v", i, a, b)
		}
	}
}

func TestLimit(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).Limit(2, "").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows with limit=2, got %d", len(rows))
	}
}

func TestRange(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	_, err := client.From("todos").Select("*", "", false).Range(0, 1, "").ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	// Range(0,1) => offset=0, limit=2 (to-from+1)
	if len(rows) != 2 {
		t.Errorf("expected 2 rows from range 0-1, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// Single / MaybeSingle
// ---------------------------------------------------------------------------

func TestSingle(t *testing.T) {
	client := newClient()
	var row map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Eq("name", "Alice").Single().ExecuteTo(&row)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if row["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", row["name"])
	}
}

func TestSingleNoRows(t *testing.T) {
	// Single() with no matching rows should return an error
	client := newClient()
	var row map[string]interface{}
	_, err := client.From("persons").Select("*", "", false).Eq("name", "NonExistent").Single().ExecuteTo(&row)
	if err == nil {
		t.Error("expected error when Single() has no rows, got nil")
	}
}

func TestMaybeSingleNoRows(t *testing.T) {
	// MaybeSingle() returns nothing but no 406 error when no rows
	h := harness.New(t)
	h.Get("/persons", harness.P("name", "eq.NonExistent"),
		harness.H_("Accept", "application/json")).
		StatusIn(200, 204)
}

// ---------------------------------------------------------------------------
// CSV output
// ---------------------------------------------------------------------------

func TestCSV(t *testing.T) {
	h := harness.New(t)
	h.Get("/todos", harness.P("select", "*"), harness.H_("Accept", "text/csv")).
		Status(200).
		ContentType("text/csv")
}

// ---------------------------------------------------------------------------
// INSERT
// ---------------------------------------------------------------------------

func TestInsert(t *testing.T) {
	client := newClient()
	h := harness.New(t)

	var inserted []map[string]interface{}
	_, err := client.From("todos").
		Insert(map[string]interface{}{"task": "test insert go"}, false, "", "representation", "").
		ExecuteTo(&inserted)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(inserted) == 0 {
		t.Fatal("expected inserted row returned")
	}

	id := inserted[0]["id"]
	idStr := jsonNumberToString(id)

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+idStr), nil).StatusIn(200, 204)
	})

	if inserted[0]["task"] != "test insert go" {
		t.Errorf("unexpected task: %v", inserted[0]["task"])
	}
}

func TestInsertBulk(t *testing.T) {
	client := newClient()
	h := harness.New(t)

	rows := []map[string]interface{}{
		{"task": "bulk insert go 1"},
		{"task": "bulk insert go 2"},
	}
	var inserted []map[string]interface{}
	_, err := client.From("todos").Insert(rows, false, "", "representation", "").ExecuteTo(&inserted)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", len(inserted))
	}

	t.Cleanup(func() {
		for _, row := range inserted {
			idStr := jsonNumberToString(row["id"])
			h.Delete("/todos", harness.P("id", "eq."+idStr), nil).StatusIn(200, 204)
		}
	})
}

func TestInsertMinimalReturn(t *testing.T) {
	client := newClient()
	h := harness.New(t)

	// Insert with minimal return — no body returned
	raw, _, err := client.From("todos").
		Insert(map[string]interface{}{"task": "minimal return go"}, false, "", "minimal", "").
		Execute()
	if err != nil {
		t.Fatalf("insert minimal: %v", err)
	}

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.minimal return go"), nil).StatusIn(200, 204)
	})

	// minimal return sends back empty or just the count
	if len(raw) > 0 {
		// Could be empty array or count; just verify not an error
		var v interface{}
		if jsonErr := json.Unmarshal(raw, &v); jsonErr != nil {
			t.Errorf("unexpected non-JSON response with minimal return: %s", raw)
		}
	}
}

// ---------------------------------------------------------------------------
// UPSERT
// ---------------------------------------------------------------------------

func TestUpsert(t *testing.T) {
	client := newClient()
	h := harness.New(t)

	var rows []map[string]interface{}
	_, err := client.From("persons").
		Upsert(map[string]interface{}{"name": "Upserted", "age": 99, "email": "upserted@example.com"}, "email", "representation", "").
		ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	t.Cleanup(func() {
		h.Delete("/persons", harness.P("email", "eq.upserted@example.com"), nil).StatusIn(200, 204)
	})

	if len(rows) == 0 {
		t.Error("expected upserted row returned")
	}
}

// ---------------------------------------------------------------------------
// UPDATE
// ---------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	client := newClient()
	h := harness.New(t)

	// Insert a row to update
	var inserted []map[string]interface{}
	_, err := client.From("todos").
		Insert(map[string]interface{}{"task": "to update go"}, false, "", "representation", "").
		ExecuteTo(&inserted)
	if err != nil || len(inserted) == 0 {
		t.Fatalf("setup insert: %v", err)
	}
	idStr := jsonNumberToString(inserted[0]["id"])

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+idStr), nil).StatusIn(200, 204)
	})

	// Update the row
	var updated []map[string]interface{}
	_, err = client.From("todos").
		Update(map[string]interface{}{"done": true}, "representation", "").
		Eq("id", idStr).
		ExecuteTo(&updated)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated) == 0 {
		t.Fatal("expected updated row returned")
	}
	done, _ := updated[0]["done"].(bool)
	if !done {
		t.Errorf("expected done=true after update, got %v", updated[0]["done"])
	}
}

// ---------------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	client := newClient()

	// Insert a row to delete
	var inserted []map[string]interface{}
	_, err := client.From("todos").
		Insert(map[string]interface{}{"task": "to delete go"}, false, "", "representation", "").
		ExecuteTo(&inserted)
	if err != nil || len(inserted) == 0 {
		t.Fatalf("setup insert: %v", err)
	}
	idStr := jsonNumberToString(inserted[0]["id"])

	// Delete it
	_, _, err = client.From("todos").Delete("minimal", "").
		Eq("id", idStr).
		Execute()
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify gone
	var rows []map[string]interface{}
	_, err = client.From("todos").Select("*", "", false).Eq("id", idStr).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("verify delete: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected row deleted, but still found: %v", rows)
	}
}

func TestDeleteWithFilter(t *testing.T) {
	client := newClient()

	// Insert two rows
	var inserted []map[string]interface{}
	_, err := client.From("todos").
		Insert([]map[string]interface{}{
			{"task": "delete filter go a"},
			{"task": "delete filter go b"},
		}, false, "", "representation", "").
		ExecuteTo(&inserted)
	if err != nil || len(inserted) != 2 {
		t.Fatalf("setup insert: %v", err)
	}

	ids := make([]string, len(inserted))
	for i, row := range inserted {
		ids[i] = jsonNumberToString(row["id"])
	}

	// Delete only the first
	_, _, err = client.From("todos").Delete("minimal", "").
		Eq("id", ids[0]).
		Execute()
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Cleanup second
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+ids[1]), nil).StatusIn(200, 204)
	})

	// Verify first is gone
	var rows []map[string]interface{}
	_, err = client.From("todos").Select("*", "", false).Eq("id", ids[0]).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("first row should be deleted")
	}
}

// ---------------------------------------------------------------------------
// RPC
// ---------------------------------------------------------------------------

func TestRpcPost(t *testing.T) {
	client := newClient()
	result := client.Rpc("add", "", map[string]interface{}{"a": 3, "b": 4})
	if client.ClientError != nil {
		t.Fatalf("Rpc error: %v", client.ClientError)
	}
	// result is "7" (JSON number)
	var n float64
	if err := json.Unmarshal([]byte(result), &n); err != nil {
		t.Fatalf("parse rpc result %q: %v", result, err)
	}
	if n != 7 {
		t.Errorf("add(3,4) = %v, want 7", n)
	}
}

func TestRpcGetTodosCount(t *testing.T) {
	h := harness.New(t)
	h.Get("/rpc/get_todos_count", nil, nil).
		Status(200)
}

func TestRpcGetPersonByName(t *testing.T) {
	client := newClient()
	result, err := client.RpcWithError("get_person_by_name", "", map[string]interface{}{"name_param": "Alice"})
	if err != nil {
		t.Fatalf("RpcWithError: %v", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &rows); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Alice" {
		t.Errorf("expected Alice from get_person_by_name, got %v", rows)
	}
}

func TestRpcAddViaPOST(t *testing.T) {
	h := harness.New(t)
	h.Post("/rpc/add", nil, nil, map[string]interface{}{"a": 10, "b": 5}).
		Status(200).
		BodyContains("15")
}

func TestRpcGetViaGET(t *testing.T) {
	h := harness.New(t)
	h.Get("/rpc/add", harness.P("a", "10", "b", "5"), nil).
		Status(200).
		BodyContains("15")
}

// ---------------------------------------------------------------------------
// Schema switching (Accept-Profile: private)
// ---------------------------------------------------------------------------

func TestSchemaSwitchingAcceptProfile(t *testing.T) {
	h := harness.New(t)
	h.Get("/items", nil, harness.H_("Accept-Profile", "private")).
		Status(200).
		ArrayLen(2)
}

func TestSchemaSwitchingChangeSchema(t *testing.T) {
	// ChangeSchema mutates the client to switch to private schema.
	// Since ChangeSchema mutates in-place, create a fresh client.
	client := postgrest.NewClient(harness.ServerURL(), "api", nil)
	client.ChangeSchema("private")

	var rows []map[string]interface{}
	_, err := client.From("items").Select("*", "", false).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo with private schema: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 items in private schema, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// Count extraction from Content-Range
// ---------------------------------------------------------------------------

func TestCountExact(t *testing.T) {
	client := newClient()
	var rows []map[string]interface{}
	count, err := client.From("todos").Select("*", "exact", false).ExecuteTo(&rows)
	if err != nil {
		t.Fatalf("ExecuteTo: %v", err)
	}
	if count < 3 {
		t.Errorf("expected count >= 3 (at least seed rows), got %d", count)
	}
}

func TestCountViaContentRange(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("select", "*"), harness.H_("Prefer", "count=exact"))
	res.Status(200)
	cr := res.Header("Content-Range")
	if cr == "" {
		t.Fatal("expected Content-Range header")
	}
	total := harness.ContentRangeTotal(cr)
	if total < 3 {
		t.Errorf("Content-Range total %d, want >= 3", total)
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestSelectNonExistentTable(t *testing.T) {
	h := harness.New(t)
	h.Get("/nonexistent_table_xyz", nil, nil).
		StatusIn(404, 406, 400)
}

func TestInsertInvalidColumn(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos", nil, harness.H_("Prefer", "return=representation"),
		map[string]interface{}{"nonexistent_col": "value"}).
		StatusIn(400, 404, 422)
}

// ---------------------------------------------------------------------------
// Embedded resources (FK joins)
// ---------------------------------------------------------------------------

func TestEmbedMessagesWithChannels(t *testing.T) {
	h := harness.New(t)
	h.Get("/messages", harness.P("select", "id,message,channels(id,slug)"), nil).
		Status(200)
}

func TestEmbedCitiesWithCountries(t *testing.T) {
	h := harness.New(t)
	h.Get("/cities", harness.P("select", "id,name,countries(id,name)"), nil).
		Status(200)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// jsonNumberToString converts a JSON number (float64 or json.Number) to
// its string representation suitable for use in query params.
func jsonNumberToString(v interface{}) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case json.Number:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

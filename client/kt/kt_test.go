// Package kt_test verifies wire compatibility with the Kotlin postgrest-kt client
// (io.supabase:postgrest-kt). Each test sends the exact HTTP request the Kotlin
// client would generate and asserts the server response.
package kt_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// KT1: Select with eq filter (property-ref style).
// client.from("messages").select("*").eq("id", 1)
// => GET /messages?id=eq.1
func TestKT1_SelectEq(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/messages", harness.P("id", "eq.1"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	id, ok := rows[0]["id"]
	if !ok {
		t.Fatal("row missing 'id' field")
	}
	if id.(float64) != 1 {
		t.Errorf("id: got %v, want 1", id)
	}
}

// KT2: Select with neq filter.
// client.from("messages").select("*").neq("message", "hello world")
// => GET /messages?message=neq.hello world
func TestKT2_SelectNeq(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/messages", harness.P("message", "neq.hello world"), nil)
	res.Status(200)
	rows := res.JSONArray()
	for i, row := range rows {
		if msg, ok := row["message"]; ok && msg == "hello world" {
			t.Errorf("row[%d] should have been filtered out (message=hello world)", i)
		}
	}
}

// KT3: Select with embedded company resource.
// client.from("people").select("name,age,companies(name,address,phone)").executeAndGetList()
// => GET /people?select=name,age,companies(name,address,phone)
func TestKT3_SelectEmbeddedCompany(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/people", harness.P("select", "name,age,companies(name,address,phone)"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	for i, row := range rows {
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing 'name'", i)
		}
		if _, ok := row["age"]; !ok {
			t.Errorf("row[%d] missing 'age'", i)
		}
		if _, ok := row["companies"]; !ok {
			t.Errorf("row[%d] missing embedded 'companies'", i)
		}
	}
}

// KT4: Select with embedded messages resource.
// client.from("persons").select("id,name,messages(id,message)").executeAndGetList()
// => GET /persons?select=id,name,messages(id,message)
func TestKT4_SelectEmbeddedMessages(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("select", "id,name,messages(id,message)"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing 'name'", i)
		}
		if _, ok := row["messages"]; !ok {
			t.Errorf("row[%d] missing embedded 'messages'", i)
		}
	}
}

// KT5: Update with eq filter.
// client.from("todos").update(mapOf("done" to true)).eq("id", 1)
// => PATCH /todos?id=eq.1  body={"done":true}
func TestKT5_UpdateWithEq(t *testing.T) {
	h := harness.New(t)

	// record original value
	origRes := h.Get("/todos", harness.P("id", "eq.1"), nil)
	origRes.Status(200)
	origRows := origRes.JSONArray()
	if len(origRows) == 0 {
		t.Fatal("todo id=1 not found")
	}
	origDone := origRows[0]["done"]

	// restore after test
	t.Cleanup(func() {
		h.Patch("/todos", harness.P("id", "eq.1"), nil, map[string]any{"done": origDone})
	})

	// update
	res := h.Patch("/todos", harness.P("id", "eq.1"), nil, map[string]any{"done": true})
	res.Status(204)

	// verify
	checkRes := h.Get("/todos", harness.P("id", "eq.1"), nil)
	checkRes.Status(200)
	checkRows := checkRes.JSONArray()
	if len(checkRows) != 1 {
		t.Fatalf("expected 1 todo after update, got %d", len(checkRows))
	}
	if checkRows[0]["done"] != true {
		t.Errorf("done: got %v, want true", checkRows[0]["done"])
	}
}

// KT6: Delete with eq filter (restore afterwards).
// client.from("messages").delete().eq("id", 4)
// => DELETE /messages?id=eq.4
func TestKT6_DeleteWithEq(t *testing.T) {
	h := harness.New(t)

	// fetch original row to restore later
	origRes := h.Get("/messages", harness.P("id", "eq.4"), nil)
	origRes.Status(200)
	origRows := origRes.JSONArray()
	if len(origRows) == 0 {
		t.Skip("message id=4 not found, skipping")
	}
	orig := origRows[0]

	// restore after test
	t.Cleanup(func() {
		h.Post("/messages", nil, harness.H_("Prefer", "return=minimal"), orig)
	})

	// delete
	res := h.Delete("/messages", harness.P("id", "eq.4"), nil)
	res.Status(204)

	// verify deleted
	checkRes := h.Get("/messages", harness.P("id", "eq.4"), nil)
	checkRes.Status(200)
	if len(checkRes.JSONArray()) != 0 {
		t.Error("expected message id=4 to be deleted")
	}
}

// KT7: Insert single row.
// client.from("messages").insert(mapOf("message" to "kt test", "channel_id" to 1, "person_id" to 1))
// => POST /messages  body={"message":"kt test","channel_id":1,"person_id":1}
func TestKT7_InsertSingle(t *testing.T) {
	h := harness.New(t)

	body := map[string]any{
		"message":    "kt test",
		"channel_id": 1,
		"person_id":  1,
	}
	res := h.Post("/messages", nil, harness.H_("Prefer", "return=representation"), body)
	res.Status(201)

	inserted := res.JSONArray()
	t.Cleanup(func() {
		for _, row := range inserted {
			if id, ok := row["id"]; ok {
				if fid, ok := id.(float64); ok {
					h.Delete("/messages", harness.P("id", fmt.Sprintf("eq.%d", int64(fid))), nil)
				}
			}
		}
	})

	if len(inserted) == 0 {
		t.Fatal("expected inserted row in response")
	}
	if inserted[0]["message"] != "kt test" {
		t.Errorf("message: got %v, want 'kt test'", inserted[0]["message"])
	}
}

// KT8: RPC get_person_by_name.
// client.rpc("get_person_by_name", mapOf("name_param" to "Alice"))
// => POST /rpc/get_person_by_name  body={"name_param":"Alice"}
func TestKT8_RpcGetPersonByName(t *testing.T) {
	h := harness.New(t)

	res := h.Post("/rpc/get_person_by_name", nil, nil, map[string]any{"name_param": "Alice"})
	res.Status(200)

	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result")
	}
	found := false
	for _, row := range rows {
		if row["name"] == "Alice" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Alice not found in results: %v", rows)
	}
}

// KT9: RPC add.
// client.rpc("add", mapOf("a" to 5, "b" to 3))
// => POST /rpc/add  body={"a":5,"b":3}  => 200, 8
func TestKT9_RpcAdd(t *testing.T) {
	h := harness.New(t)

	res := h.Post("/rpc/add", nil, nil, map[string]any{"a": 5, "b": 3})
	res.Status(200)

	var result float64
	if err := json.Unmarshal(res.RawBody(), &result); err != nil {
		t.Fatalf("parse result: %v (body: %s)", err, res.RawBody())
	}
	if result != 8 {
		t.Errorf("add(5,3): got %v, want 8", result)
	}
}

// KT10: Bulk insert list.
// client.from("todos").insert(listOf(mapOf("task" to "kt bulk 1"), mapOf("task" to "kt bulk 2")))
// => POST /todos  body=[{"task":"kt bulk 1"},{"task":"kt bulk 2"}]
func TestKT10_BulkInsert(t *testing.T) {
	h := harness.New(t)

	body := []map[string]any{
		{"task": "kt bulk 1"},
		{"task": "kt bulk 2"},
	}
	res := h.Post("/todos", nil, harness.H_("Prefer", "return=representation"), body)
	res.Status(201)

	inserted := res.JSONArray()
	t.Cleanup(func() {
		for _, row := range inserted {
			if id, ok := row["id"]; ok {
				if fid, ok := id.(float64); ok {
					h.Delete("/todos", harness.P("id", fmt.Sprintf("eq.%d", int64(fid))), nil)
				}
			}
		}
	})

	if len(inserted) != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", len(inserted))
	}
}

// KT11: Filter gt.
// client.from("persons").select("*").gt("age", 25)
// => GET /persons?age=gt.25
func TestKT11_FilterGt(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "gt.25"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one person with age > 25")
	}
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		if age.(float64) <= 25 {
			t.Errorf("row[%d] age %v should be > 25", i, age)
		}
	}
}

// KT12: Filter lte.
// client.from("persons").select("*").lte("age", 30)
// => GET /persons?age=lte.30
func TestKT12_FilterLte(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "lte.30"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result")
	}
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		if age.(float64) > 30 {
			t.Errorf("row[%d] age %v should be <= 30", i, age)
		}
	}
}

// KT13: Filter like.
// client.from("todos").select("*").like("task", "*laundry*")
// => GET /todos?task=like.*laundry*
func TestKT13_FilterLike(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "like.*laundry*"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result matching 'laundry'")
	}
	for _, row := range rows {
		task, _ := row["task"].(string)
		if !containsStr(task, "laundry") {
			t.Errorf("task %q does not contain 'laundry'", task)
		}
	}
}

// KT14: Filter fts (full text search).
// client.from("todos").select("*").fts("task", "tutorial")
// => GET /todos?task=fts.tutorial
func TestKT14_FilterFts(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result matching 'tutorial'")
	}
}

// KT15: Filter in.
// client.from("channels").select("*").filter("id", "in", "(1,2)")
// => GET /channels?id=in.(1,2)
func TestKT15_FilterIn(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/channels", harness.P("id", "in.(1,2)"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(rows))
	}
	for i, row := range rows {
		id, ok := row["id"]
		if !ok {
			t.Errorf("row[%d] missing 'id'", i)
			continue
		}
		fid := id.(float64)
		if fid != 1 && fid != 2 {
			t.Errorf("row[%d] id %v not in (1,2)", i, fid)
		}
	}
}

// KT16: or filter.
// client.from("todos").select("*").or("done.eq.true,id.eq.3")
// => GET /todos?or=(done.eq.true,id.eq.3)
func TestKT16_OrFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("or", "(done.eq.true,id.eq.3)")
	res := h.Get("/todos", params, nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result")
	}
	for i, row := range rows {
		done, _ := row["done"].(bool)
		id, _ := row["id"].(float64)
		if !done && id != 3 {
			t.Errorf("row[%d] does not satisfy done=true OR id=3: %v", i, row)
		}
	}
}

// KT17: executeAndGetSingle via Accept header.
// client.from("todos").select("*").eq("id", 1).executeAndGetSingle()
// sends Accept: application/vnd.pgrst.object+json to receive a single object
// => GET /todos?id=eq.1  Accept: application/vnd.pgrst.object+json
func TestKT17_ExecuteAndGetSingle(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	res.Status(200)
	obj := res.JSONObject()
	if obj["id"].(float64) != 1 {
		t.Errorf("id: got %v, want 1", obj["id"])
	}
}

// KT18: Schema switch via Accept-Profile header.
// client configured with schema="private"
// => GET /items  Accept-Profile: private
func TestKT18_SchemaSwitch(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/items", nil, harness.H_("Accept-Profile", "private"))
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one item in private schema")
	}
	for i, row := range rows {
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing 'name'", i)
		}
	}
}

// containsStr reports whether s contains sub.
func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

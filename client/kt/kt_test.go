// Package kt_test verifies wire compatibility with the Kotlin postgrest-kt client
// (io.supabase:postgrest-kt). Each test sends the exact HTTP request the Kotlin
// client would generate and asserts the server response.
package kt_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

// makeAnonJWT builds a signed HS256 JWT with role=web_anon.
func makeAnonJWT() string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pay := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"web_anon"}`))
	msg := hdr + "." + pay
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// KT19: Filter gte.
// GET /persons?age=gte.30 => all rows with age >= 30
func TestKT19_FilterGte(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "gte.30"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one person with age >= 30")
	}
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		if age.(float64) < 30 {
			t.Errorf("row[%d] age %v should be >= 30", i, age)
		}
	}
}

// KT20: Filter lt.
// GET /persons?age=lt.30 => all rows with age < 30
func TestKT20_FilterLt(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "lt.30"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one person with age < 30")
	}
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		if age.(float64) >= 30 {
			t.Errorf("row[%d] age %v should be < 30", i, age)
		}
	}
}

// KT21: Filter ilike (case-insensitive).
// GET /todos?task=ilike.*LAUNDRY* => at least 1 row
func TestKT21_FilterIlike(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "ilike.*LAUNDRY*"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result matching LAUNDRY (case-insensitive)")
	}
}

// KT22: Filter is null.
// GET /todos?due=is.null => only rows where due is null
func TestKT22_FilterIsNull(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("due", "is.null"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one todo with null due (id=2)")
	}
	for i, row := range rows {
		if row["due"] != nil {
			t.Errorf("row[%d] due should be null, got %v", i, row["due"])
		}
	}
}

// KT23: Filter not.
// GET /todos?done=not.eq.true => all rows where done is false
func TestKT23_FilterNot(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("done", "not.eq.true"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one todo with done=false")
	}
	for i, row := range rows {
		if row["done"] == true {
			t.Errorf("row[%d] done should not be true", i)
		}
	}
}

// KT24: Order ascending.
// GET /persons?order=age.asc => rows in ascending age order
func TestKT24_OrderAsc(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("order", "age.asc"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	var prev float64 = -1
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		a := age.(float64)
		if a < prev {
			t.Errorf("row[%d] age %v is less than previous %v (not ascending)", i, a, prev)
		}
		prev = a
	}
}

// KT25: Order descending.
// GET /persons?order=age.desc => rows in descending age order
func TestKT25_OrderDesc(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("order", "age.desc"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	var prev float64 = 1<<53 - 1
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		a := age.(float64)
		if a > prev {
			t.Errorf("row[%d] age %v is greater than previous %v (not descending)", i, a, prev)
		}
		prev = a
	}
}

// KT26: Order nulls first.
// GET /todos?order=due.asc.nullsfirst => first row has due=null
func TestKT26_OrderNullsFirst(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("order", "due.asc.nullsfirst"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	if rows[0]["due"] != nil {
		t.Errorf("first row due should be null (nullsfirst), got %v", rows[0]["due"])
	}
}

// KT27: Limit.
// GET /persons?limit=1 => exactly 1 row
func TestKT27_Limit(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("limit", "1"), nil)
	res.Status(200)
	res.ArrayLen(1)
}

// KT28: Offset.
// GET /persons?order=id.asc&offset=1 => does not include id=1
func TestKT28_Offset(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("order", "id.asc", "offset", "1"), nil)
	res.Status(200)
	rows := res.JSONArray()
	for i, row := range rows {
		if row["id"].(float64) == 1 {
			t.Errorf("row[%d] id=1 should have been skipped by offset=1", i)
		}
	}
}

// KT29: Limit + offset.
// GET /persons?order=id.asc&limit=1&offset=1 => exactly 1 row
func TestKT29_LimitOffset(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("order", "id.asc", "limit", "1", "offset", "1"), nil)
	res.Status(200)
	res.ArrayLen(1)
}

// KT30: Range header.
// GET /persons Range: 0-1 => 200 or 206, at most 2 rows
func TestKT30_RangeHeader(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", nil, harness.H_("Range", "0-1"))
	res.StatusIn(200, 206)
	rows := res.JSONArray()
	if len(rows) > 2 {
		t.Errorf("expected at most 2 rows with Range: 0-1, got %d", len(rows))
	}
}

// KT31: Count exact.
// GET /todos Prefer: count=exact => Content-Range total = 3
func TestKT31_CountExact(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", nil, harness.H_("Prefer", "count=exact"))
	res.Status(200)
	res.HasHeader("Content-Range")
	cr := res.Header("Content-Range")
	if total := harness.ContentRangeTotal(cr); total != 3 {
		t.Errorf("Content-Range total: got %d, want 3 (cr=%q)", total, cr)
	}
}

// KT32: Count planned.
// GET /todos Prefer: count=planned => 200 or 206, has Content-Range
func TestKT32_CountPlanned(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", nil, harness.H_("Prefer", "count=planned"))
	res.StatusIn(200, 206)
	res.HasHeader("Content-Range")
}

// KT33: Count estimated.
// GET /todos Prefer: count=estimated => 200 or 206, has Content-Range
func TestKT33_CountEstimated(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", nil, harness.H_("Prefer", "count=estimated"))
	res.StatusIn(200, 206)
	res.HasHeader("Content-Range")
}

// KT34: HEAD count exact.
// HEAD /todos Prefer: count=exact => 200, has Content-Range, empty body
func TestKT34_HeadCount(t *testing.T) {
	h := harness.New(t)
	res := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	res.Status(200)
	res.HasHeader("Content-Range")
	res.EmptyBody()
}

// KT35: Insert return=minimal.
// POST /todos Prefer: return=minimal => 201, no body
func TestKT35_InsertReturnMinimal(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos", nil, harness.H_("Prefer", "return=minimal"), map[string]any{"task": "kt-kt35-minimal"})
	res.Status(201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.kt-kt35-minimal"), nil)
	})
}

// KT36: Insert return=headers-only.
// POST /todos Prefer: return=headers-only => 201, has Location header
func TestKT36_InsertReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos", nil, harness.H_("Prefer", "return=headers-only"), map[string]any{"task": "kt-kt36-headersonly"})
	res.Status(201)
	res.HasHeader("Location")
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.kt-kt36-headersonly"), nil)
	})
}

// KT37: Upsert.
// POST /todos?on_conflict=task Prefer: resolution=merge-duplicates,return=representation => 200 or 201
func TestKT37_Upsert(t *testing.T) {
	h := harness.New(t)
	res := h.Post(
		"/todos",
		harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=merge-duplicates,return=representation"),
		map[string]any{"task": "kt-kt37-upsert", "done": false},
	)
	res.StatusIn(200, 201)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.kt-kt37-upsert"), nil)
	})
}

// KT38: Update return=representation.
// Insert a row, PATCH Prefer: return=representation => 200, non-empty array
func TestKT38_UpdateReturnRepresentation(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos", nil, harness.H_("Prefer", "return=representation"), map[string]any{"task": "kt-kt38-update"})
	ins.Status(201)
	rows := ins.JSONArray()
	if len(rows) == 0 {
		t.Fatal("insert returned no rows")
	}
	id := rows[0]["id"].(float64)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", fmt.Sprintf("eq.%d", int64(id))), nil)
	})

	res := h.Patch(
		"/todos",
		harness.P("id", fmt.Sprintf("eq.%d", int64(id))),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"done": true},
	)
	res.Status(200)
	updated := res.JSONArray()
	if len(updated) == 0 {
		t.Error("expected non-empty array in PATCH response")
	}
}

// KT39: Delete return=representation.
// Insert a row, DELETE Prefer: return=representation => 200, non-empty body
func TestKT39_DeleteReturnRepresentation(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos", nil, harness.H_("Prefer", "return=representation"), map[string]any{"task": "kt-kt39-delete"})
	ins.Status(201)
	rows := ins.JSONArray()
	if len(rows) == 0 {
		t.Fatal("insert returned no rows")
	}
	id := rows[0]["id"].(float64)

	res := h.Delete(
		"/todos",
		harness.P("id", fmt.Sprintf("eq.%d", int64(id))),
		harness.H_("Prefer", "return=representation"),
	)
	res.Status(200)
	deleted := res.JSONArray()
	if len(deleted) == 0 {
		t.Error("expected non-empty body in DELETE response")
	}
}

// KT40: Select column alias.
// GET /messages?select=msgId:id,message => rows have "msgId" key
func TestKT40_SelectColumnAlias(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/messages", harness.P("select", "msgId:id,message"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one message")
	}
	for i, row := range rows {
		if _, ok := row["msgId"]; !ok {
			t.Errorf("row[%d] missing aliased field 'msgId'", i)
		}
		if _, ok := row["id"]; ok {
			t.Errorf("row[%d] should not have original 'id' when aliased to 'msgId'", i)
		}
	}
}

// KT41: Select specific columns only.
// GET /persons?select=id,name => rows have id+name, no age
func TestKT41_SelectColumns(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("select", "id,name"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one person")
	}
	for i, row := range rows {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing 'id'", i)
		}
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing 'name'", i)
		}
		if _, ok := row["age"]; ok {
			t.Errorf("row[%d] should not have 'age' when not selected", i)
		}
	}
}

// KT42: And filter.
// GET /persons?and=(age.gte.25,age.lte.30) => 25 <= age <= 30
func TestKT42_AndFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("and", "(age.gte.25,age.lte.30)")
	res := h.Get("/persons", params, nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one person with 25 <= age <= 30")
	}
	for i, row := range rows {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing 'age'", i)
			continue
		}
		a := age.(float64)
		if a < 25 || a > 30 {
			t.Errorf("row[%d] age %v should be between 25 and 30", i, a)
		}
	}
}

// KT43: Or filter on embedded resource.
// GET /channels?select=id,messages(id,message)&messages.or=(channel_id.eq.1,channel_id.eq.2) => 200
func TestKT43_OrFilterEmbedded(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,messages(id,message)")
	params.Set("messages.or", "(channel_id.eq.1,channel_id.eq.2)")
	res := h.Get("/channels", params, nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one channel")
	}
}

// KT44: RPC GET.
// GET /rpc/get_todos_count => 200
func TestKT44_RpcGet(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/rpc/get_todos_count", nil, nil)
	res.Status(200)
}

// KT45: RPC POST with count=exact.
// POST /rpc/get_todos_count Prefer: count=exact body={} => 200
func TestKT45_RpcCount(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/rpc/get_todos_count", nil, harness.H_("Prefer", "count=exact"), map[string]any{})
	res.Status(200)
}

// KT46: JWT auth.
// GET /todos Authorization: Bearer <valid jwt role=web_anon> => 200
func TestKT46_JwtAuth(t *testing.T) {
	h := harness.New(t)
	token := makeAnonJWT()
	res := h.Get("/todos", nil, harness.H_("Authorization", "Bearer "+token))
	res.Status(200)
}

// KT47: Content-Profile header for schema routing.
// POST /items Content-Profile: private Prefer: return=minimal => 201 or 403
func TestKT47_ContentProfile(t *testing.T) {
	h := harness.New(t)
	res := h.Post(
		"/items",
		nil,
		harness.H_("Content-Profile", "private", "Prefer", "return=minimal"),
		map[string]any{"name": "kt-kt47-item"},
	)
	res.StatusIn(201, 401, 403)
	if res.Header("") != "" {
		// cleanup only if inserted
	}
	t.Cleanup(func() {
		h.Delete("/items", harness.P("name", "eq.kt-kt47-item"), harness.H_("Accept-Profile", "private"))
	})
}

// KT48: Filter plfts (plain full text search).
// GET /todos?task=plfts.tutorial => 200
func TestKT48_FilterPlfts(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "plfts.tutorial"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result from plfts.tutorial")
	}
}

// KT49: Filter phfts (phrase full text search).
// GET /todos?task=phfts.finish tutorial => 200
func TestKT49_FilterPhfts(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "phfts.finish tutorial"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result from phfts.finish tutorial")
	}
}

// KT50: Filter wfts (websearch full text search).
// GET /todos?task=wfts.tutorial => 200
func TestKT50_FilterWfts(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "wfts.tutorial"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one result from wfts.tutorial")
	}
}

// KT51: Filter cs (contains).
// GET /todos?tags=cs.{go} => at least 1 row (id=1 has go in tags)
func TestKT51_FilterCs(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("tags", "cs.{go}"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row with tags containing 'go'")
	}
}

// KT52: Filter cd (contained by).
// GET /todos?tags=cd.{go,sql,pets,chores,home} => 200
func TestKT52_FilterCd(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("tags", "cd.{go,sql,pets,chores,home}"), nil)
	res.Status(200)
}

// KT53: Filter ov (overlap).
// GET /todos?tags=ov.{go,pets} => id=1 (go) and id=2 (pets) match
func TestKT53_FilterOv(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("tags", "ov.{go,pets}"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows (id=1 go, id=2 pets), got %d", len(rows))
	}
}

// KT54: CSV output.
// GET /todos Accept: text/csv => 200, Content-Type: text/csv
func TestKT54_CsvOutput(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", nil, harness.H_("Accept", "text/csv"))
	res.Status(200)
	res.ContentType("text/csv")
}

// KT55: Maybe-single no rows => 406.
// GET /todos?id=eq.9999 Accept: application/vnd.pgrst.object+json => 406
func TestKT55_MaybeSingleNoRows(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("id", "eq.9999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"))
	res.Status(406)
}

// KT56: Single with multiple rows => 406.
// GET /todos Accept: application/vnd.pgrst.object+json (no id filter) => 406
func TestKT56_SingleMultipleRows406(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", nil, harness.H_("Accept", "application/vnd.pgrst.object+json"))
	res.Status(406)
}

// KT57: Filter on column that might not exist (email).
// GET /persons?email=eq.alice@example.com => 200
func TestKT57_FilterGeneric(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("email", "eq.alice@example.com"), nil)
	res.Status(200)
}

// KT58: Order nulls last.
// GET /todos?order=due.asc.nullslast => last row has due=null
func TestKT58_OrderNullsLast(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("order", "due.asc.nullslast"), nil)
	res.Status(200)
	rows := res.JSONArray()
	if len(rows) == 0 {
		t.Fatal("expected at least one row")
	}
	last := rows[len(rows)-1]
	if last["due"] != nil {
		t.Errorf("last row due should be null (nullslast), got %v", last["due"])
	}
}

// _ is a compile-time check that the strings import is used.
var _ = strings.Contains

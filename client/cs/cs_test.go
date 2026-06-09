// Package cs_test replicates the exact HTTP wire traffic produced by the
// C# Supabase.Postgrest client (NuGet package Supabase.Postgrest v4+).
//
// Key wire protocol differences from JS:
//   - ALWAYS sends Prefer: return=representation on writes (C# default)
//   - PascalCase model properties map to snake_case column names
//   - LINQ expressions translate to PostgREST filter syntax
//   - Tags.Contains("Action") => ?tags=cs.{Action}  (array containment)
package cs_test

import (
	"net/url"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// CS1: LINQ eq filter - done todos
func TestCS1_LINQEqTrue(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("done", "eq.true"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		done, ok := row["done"]
		if !ok {
			t.Errorf("row[%d] missing field 'done'", i)
			continue
		}
		if done != true {
			t.Errorf("row[%d] done=%v, want true", i, done)
		}
	}
}

// CS2: LINQ neq filter - undone todos
func TestCS2_LINQNeqFalse(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("done", "neq.true"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		done, ok := row["done"]
		if !ok {
			t.Errorf("row[%d] missing field 'done'", i)
			continue
		}
		if done != false {
			t.Errorf("row[%d] done=%v, want false", i, done)
		}
	}
}

// CS3: LINQ gt filter - persons age > 25
func TestCS3_LINQGtAge(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "gt.25"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing field 'age'", i)
			continue
		}
		ageF, _ := age.(float64)
		if ageF <= 25 {
			t.Errorf("row[%d] age=%v, want > 25", i, age)
		}
	}
}

// CS4: LINQ gte filter - persons age >= 30
func TestCS4_LINQGteAge(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "gte.30"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing field 'age'", i)
			continue
		}
		ageF, _ := age.(float64)
		if ageF < 30 {
			t.Errorf("row[%d] age=%v, want >= 30", i, age)
		}
	}
}

// CS5: LINQ lt filter - persons age < 35
func TestCS5_LINQLtAge(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "lt.35"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing field 'age'", i)
			continue
		}
		ageF, _ := age.(float64)
		if ageF >= 35 {
			t.Errorf("row[%d] age=%v, want < 35", i, age)
		}
	}
}

// CS6: LINQ select columns - only id,name,tags,release_date returned
func TestCS6_LINQSelectColumns(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/movies", harness.P("select", "id,name,tags,release_date"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one movie")
	}
	for i, row := range arr {
		for _, field := range []string{"id", "name", "tags", "release_date"} {
			if _, ok := row[field]; !ok {
				t.Errorf("row[%d] missing field %q", i, field)
			}
		}
		// watched_at should NOT be present
		if _, ok := row["watched_at"]; ok {
			t.Errorf("row[%d] unexpected field 'watched_at'", i)
		}
	}
}

// CS7: LINQ order desc - movies sorted by release_date descending
func TestCS7_LINQOrderDesc(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/movies", harness.P("select", "*", "order", "release_date.desc"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) < 2 {
		t.Skip("need at least 2 movies")
	}
	for i := 1; i < len(arr); i++ {
		prev, _ := arr[i-1]["release_date"].(string)
		curr, _ := arr[i]["release_date"].(string)
		if prev != "" && curr != "" && prev < curr {
			t.Errorf("movies not sorted desc: row[%d]=%s > row[%d]=%s", i-1, prev, i, curr)
		}
	}
}

// CS8: LINQ limit + offset - 2 movies starting at offset 1
func TestCS8_LINQLimitOffset(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/movies", harness.P("select", "*", "limit", "2", "offset", "1"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) > 2 {
		t.Errorf("expected at most 2 movies, got %d", len(arr))
	}
}

// CS9: LINQ Tags.Contains (array cs) - movies with sci-fi tag
func TestCS9_LINQArrayContains(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/movies", harness.P("tags", "cs.{sci-fi}"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one sci-fi movie")
	}
	for i, row := range arr {
		tags, ok := row["tags"]
		if !ok {
			t.Errorf("row[%d] missing field 'tags'", i)
			continue
		}
		tagsSlice, _ := tags.([]any)
		found := false
		for _, tag := range tagsSlice {
			if tag == "sci-fi" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("row[%d] tags=%v does not contain 'sci-fi'", i, tags)
		}
	}
}

// CS10: LINQ Filter Operator.Like - movies with name matching *Matrix*
func TestCS10_FilterLike(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/movies", harness.P("name", "like.*Matrix*"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one movie matching *Matrix*")
	}
	for i, row := range arr {
		name, _ := row["name"].(string)
		found := false
		for _, c := range []string{"Matrix", "matrix", "MATRIX"} {
			_ = c
			// case-sensitive like: only exact case matches
			if contains(name, "Matrix") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("row[%d] name=%q does not match *Matrix*", i, name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// CS11: LINQ Filter Operator.ILike - todos with task matching *TUTORIAL* (case insensitive)
func TestCS11_FilterILike(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "ilike.*TUTORIAL*"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one todo matching *TUTORIAL* (ilike)")
	}
	for i, row := range arr {
		task, _ := row["task"].(string)
		if !containsInsensitive(task, "tutorial") {
			t.Errorf("row[%d] task=%q does not match *TUTORIAL* ilike", i, task)
		}
	}
}

func containsInsensitive(s, sub string) bool {
	sLower := toLower(s)
	subLower := toLower(sub)
	return contains(sLower, subLower)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

// CS12: Insert + return=representation
// POST /todos?select=id,task Prefer: return=representation
// C# default: always return=representation on writes
func TestCS12_InsertReturnRepresentation(t *testing.T) {
	h := harness.New(t)

	body := map[string]any{"task": "cs test"}
	res := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		body,
	)
	res.Status(201)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected inserted row in response body")
	}
	row := arr[0]
	if row["task"] != "cs test" {
		t.Errorf("task: got %v, want 'cs test'", row["task"])
	}
	idVal, ok := row["id"]
	if !ok {
		t.Fatal("inserted row missing 'id'")
	}

	// cleanup
	t.Cleanup(func() {
		idNum := int(idVal.(float64))
		h.Delete("/todos", harness.P("id", "eq."+itoa(idNum)), nil)
	})
}

// CS13: Update + return=representation
// PATCH /persons?id=eq.1 Prefer: return=representation body={"age":31}
func TestCS13_UpdateReturnRepresentation(t *testing.T) {
	h := harness.New(t)

	body := map[string]any{"age": 31}
	res := h.Patch("/persons",
		harness.P("id", "eq.1"),
		harness.H_("Prefer", "return=representation"),
		body,
	)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected updated row in response body")
	}
	row := arr[0]
	age, _ := row["age"].(float64)
	if age != 31 {
		t.Errorf("age: got %v, want 31", row["age"])
	}

	// restore original value
	t.Cleanup(func() {
		h.Patch("/persons",
			harness.P("id", "eq.1"),
			harness.H_("Prefer", "return=representation"),
			map[string]any{"age": 30},
		)
	})
}

// CS14: Delete + return=representation
// DELETE /messages?id=eq.4 Prefer: return=representation
func TestCS14_DeleteReturnRepresentation(t *testing.T) {
	h := harness.New(t)

	// First ensure message 4 exists by inserting it if it doesn't
	// We'll insert a message and then delete it
	insertRes := h.Post("/messages",
		harness.P("select", "id,message"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"message": "to be deleted", "channel_id": 1, "person_id": 1},
	)
	insertRes.Status(201)
	insertedArr := insertRes.JSONArray()
	if len(insertedArr) == 0 {
		t.Fatal("failed to insert test message")
	}
	msgID := int(insertedArr[0]["id"].(float64))

	res := h.Delete("/messages",
		harness.P("id", "eq."+itoa(msgID)),
		harness.H_("Prefer", "return=representation"),
	)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected deleted row in response body")
	}
	row := arr[0]
	if row["message"] != "to be deleted" {
		t.Errorf("message: got %v, want 'to be deleted'", row["message"])
	}
}

// CS15: Upsert with resolution+return
// POST /todos Prefer: resolution=merge-duplicates,return=representation
func TestCS15_UpsertMergeDuplicates(t *testing.T) {
	h := harness.New(t)

	// Insert with a specific id, then upsert the same row
	insertRes := h.Post("/todos",
		harness.P("select", "id,task,done"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "upsert me", "done": false},
	)
	insertRes.Status(201)
	insertedArr := insertRes.JSONArray()
	if len(insertedArr) == 0 {
		t.Fatal("failed to insert initial todo for upsert test")
	}
	todoID := int(insertedArr[0]["id"].(float64))

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", "eq."+itoa(todoID)), nil)
	})

	// Upsert: same id, updated done=true
	upsertRes := h.Post("/todos",
		nil,
		harness.H_("Prefer", "resolution=merge-duplicates,return=representation"),
		map[string]any{"id": todoID, "task": "upsert me", "done": true},
	)
	upsertRes.StatusIn(200, 201)
	arr := upsertRes.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected upserted row in response body")
	}
}

// CS16: FK embed companies - GET /people?select=*,companies(*)
func TestCS16_FKEmbedCompanies(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/people", harness.P("select", "*,companies(*)"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one person")
	}
	for i, row := range arr {
		if _, ok := row["companies"]; !ok {
			t.Errorf("row[%d] missing embedded 'companies'", i)
		}
	}
}

// CS17: FK embed messages - GET /persons?select=id,name,messages(id,message)
func TestCS17_FKEmbedMessages(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("select", "id,name,messages(id,message)"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one person")
	}
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing field 'id'", i)
		}
		if _, ok := row["name"]; !ok {
			t.Errorf("row[%d] missing field 'name'", i)
		}
		if _, ok := row["messages"]; !ok {
			t.Errorf("row[%d] missing embedded 'messages'", i)
		}
	}
}

// CS18: Count with exact - GET /todos?select=* Prefer: count=exact
func TestCS18_CountExact(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Prefer", "count=exact"),
	)
	res.Status(200)
	res.HasHeader("Content-Range")
	cr := res.Header("Content-Range")
	total := harness.ContentRangeTotal(cr)
	if total < 0 {
		t.Errorf("Content-Range total not present or unknown: %q", cr)
	}
	if total < 3 {
		t.Errorf("expected at least 3 todos, Content-Range total=%d", total)
	}
}

// CS19: WatchedAt set - PATCH /movies?name=eq.Interstellar Prefer: return=representation
// body={"watched_at":"2024-01-01T00:00:00Z"}
func TestCS19_SetWatchedAt(t *testing.T) {
	h := harness.New(t)

	body := map[string]any{"watched_at": "2024-01-01T00:00:00Z"}
	res := h.Patch("/movies",
		harness.P("name", "eq.Interstellar"),
		harness.H_("Prefer", "return=representation"),
		body,
	)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected updated row in response body")
	}
	row := arr[0]
	if _, ok := row["watched_at"]; !ok {
		t.Errorf("updated row missing field 'watched_at'")
	}

	// restore watched_at to null
	t.Cleanup(func() {
		h.Patch("/movies",
			harness.P("name", "eq.Interstellar"),
			harness.H_("Prefer", "return=representation"),
			map[string]any{"watched_at": nil},
		)
	})
}

// CS20: Filter with nested array cs compound - GET /movies?tags=cs.{action,adventure}
// Should return Mad Max: Fury Road which has both action and adventure tags
func TestCS20_ArrayContainsCompound(t *testing.T) {
	h := harness.New(t)

	// Use url.Values directly to ensure the value is correctly encoded
	params := url.Values{}
	params.Set("tags", "cs.{action,adventure}")
	res := h.Get("/movies", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one movie with action+adventure tags")
	}
	found := false
	for _, row := range arr {
		name, _ := row["name"].(string)
		if name == "Mad Max: Fury Road" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Mad Max: Fury Road' in results, got: %v", arr)
	}
	// all returned movies must have both action and adventure tags
	for i, row := range arr {
		tags, _ := row["tags"].([]any)
		hasAction := false
		hasAdventure := false
		for _, tag := range tags {
			if tag == "action" {
				hasAction = true
			}
			if tag == "adventure" {
				hasAdventure = true
			}
		}
		if !hasAction || !hasAdventure {
			t.Errorf("row[%d] tags=%v does not contain both action and adventure", i, tags)
		}
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
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

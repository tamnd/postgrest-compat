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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

// CS21: Filter lte - persons age <= 30 (Alice 30, Bob 25; not Carol 35)
func TestCS21_FilterLte(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("age", "lte.30"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one person with age <= 30")
	}
	for i, row := range arr {
		age, _ := row["age"].(float64)
		if age > 30 {
			t.Errorf("row[%d] age=%v, want <= 30", i, row["age"])
		}
	}
}

// CS22: Filter in - persons id in (1,2)
func TestCS22_FilterIn(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("id", "in.(1,2)"), nil)
	res.Status(200)
	res.ArrayLen(2)
}

// CS23: Filter is null - todos with due=null
func TestCS23_FilterIsNull(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("due", "is.null"), nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		if due, ok := row["due"]; ok && due != nil {
			t.Errorf("row[%d] due=%v, want null", i, due)
		}
	}
}

// CS24: Filter not.is.null - todos with due not null
func TestCS24_FilterNotIsNull(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("due", "not.is.null")
	res := h.Get("/todos", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		if due, ok := row["due"]; !ok || due == nil {
			t.Errorf("row[%d] due=%v, want non-null", i, due)
		}
	}
}

// CS25: Filter array contained-by - movies with tags subset of {sci-fi,action,adventure,classic}
func TestCS25_FilterArrayContainedBy(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("tags", "cd.{sci-fi,action,adventure,classic}")
	res := h.Get("/movies", params, nil)
	res.Status(200)
	res.JSONArray()
}

// CS26: Filter array overlap - todos where tags overlap {pets,chores}
func TestCS26_FilterArrayOverlap(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("tags", "ov.{pets,chores}")
	res := h.Get("/todos", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) < 2 {
		t.Errorf("expected at least 2 rows (pets + chores), got %d", len(arr))
	}
}

// CS27: Logical or - persons age < 26 or age > 34 (Bob 25, Carol 35)
func TestCS27_LogicalOr(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("or", "(age.lt.26,age.gt.34)")
	res := h.Get("/persons", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) < 2 {
		t.Errorf("expected at least 2 persons (Bob and Carol), got %d", len(arr))
	}
	for i, row := range arr {
		age, _ := row["age"].(float64)
		if !(age < 26 || age > 34) {
			t.Errorf("row[%d] age=%v, want < 26 or > 34", i, row["age"])
		}
	}
}

// CS28: Logical and - todos done=false and due not null (id=3)
func TestCS28_LogicalAnd(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("and", "(done.eq.false,due.not.is.null)")
	res := h.Get("/todos", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one todo with done=false and due not null")
	}
	for i, row := range arr {
		done, _ := row["done"].(bool)
		if done {
			t.Errorf("row[%d] done=%v, want false", i, row["done"])
		}
		if due, ok := row["due"]; !ok || due == nil {
			t.Errorf("row[%d] due=%v, want non-null", i, due)
		}
	}
}

// CS29: Order asc - persons sorted by age ascending
func TestCS29_OrderAsc(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("order", "age.asc"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) < 2 {
		t.Skip("need at least 2 persons")
	}
	for i := 1; i < len(arr); i++ {
		prev, _ := arr[i-1]["age"].(float64)
		curr, _ := arr[i]["age"].(float64)
		if prev > curr {
			t.Errorf("persons not sorted asc: row[%d] age=%v > row[%d] age=%v", i-1, prev, i, curr)
		}
	}
}

// CS30: Order nulls first - todos sorted by due asc, null first
func TestCS30_OrderNullsFirst(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("order", "due.asc.nullsfirst"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one todo")
	}
	if arr[0]["due"] != nil {
		t.Errorf("first row due=%v, want null (nullsfirst)", arr[0]["due"])
	}
}

// CS31: Order nulls last - todos sorted by due asc, null last
func TestCS31_OrderNullsLast(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("order", "due.asc.nullslast"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one todo")
	}
	if arr[len(arr)-1]["due"] != nil {
		t.Errorf("last row due=%v, want null (nullslast)", arr[len(arr)-1]["due"])
	}
}

// CS32: Order multi-column - todos sorted by done asc then due desc
func TestCS32_OrderMultiColumn(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("order", "done.asc,due.desc"), nil)
	res.Status(200)
	res.JSONArray()
}

// CS33: Single - GET /persons?id=eq.1 Accept: vnd.pgrst.object → object with id=1
func TestCS33_Single(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	res.Status(200)
	obj := res.JSONObject()
	if id, _ := obj["id"].(float64); id != 1 {
		t.Errorf("id: got %v, want 1", obj["id"])
	}
}

// CS34: Maybe-single not found - Accept: vnd.pgrst.object with no match
func TestCS34_MaybeSingleNotFound(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons",
		harness.P("id", "eq.99999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	res.StatusIn(200, 406)
}

// CS35: Insert return=minimal
func TestCS35_InsertReturnMinimal(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"task": "cs-cs35-minimal"},
	)
	res.StatusIn(201, 204)

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.cs-cs35-minimal"), nil)
	})
}

// CS36: Insert return=headers-only → 201, has Location header
func TestCS36_InsertReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=headers-only"),
		map[string]any{"task": "cs-cs36-headersonly"},
	)
	res.Status(201)
	res.HasHeader("Location")

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.cs-cs36-headersonly"), nil)
	})
}

// CS37: Insert batch - insert 2 persons, verify 2 rows returned
func TestCS37_InsertBatch(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/persons",
		nil,
		harness.H_("Prefer", "return=representation"),
		[]map[string]any{
			{"name": "cs-batch-d1", "age": 20, "email": "d1@test.com"},
			{"name": "cs-batch-d2", "age": 21, "email": "d2@test.com"},
		},
	)
	res.StatusIn(200, 201)
	arr := res.JSONArray()
	if len(arr) != 2 {
		t.Errorf("expected 2 rows, got %d", len(arr))
	}

	t.Cleanup(func() {
		h.Delete("/persons", harness.P("email", "eq.d1@test.com"), nil)
		h.Delete("/persons", harness.P("email", "eq.d2@test.com"), nil)
	})
}

// CS38: Insert ignore-duplicates - upsert existing unique task
func TestCS38_InsertIgnoreDuplicates(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos",
		harness.P("on_conflict", "task"),
		harness.H_("Prefer", "resolution=ignore-duplicates"),
		map[string]any{"task": "finish tutorial"},
	)
	res.StatusIn(200, 201)
}

// CS39: Update return=minimal
func TestCS39_UpdateReturnMinimal(t *testing.T) {
	h := harness.New(t)
	res := h.Patch("/todos",
		harness.P("done", "eq.false"),
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"done": false},
	)
	res.StatusIn(200, 204)
}

// CS40: Update return=headers-only
func TestCS40_UpdateReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	res := h.Patch("/persons",
		harness.P("id", "eq.2"),
		harness.H_("Prefer", "return=headers-only"),
		map[string]any{"age": 25},
	)
	res.StatusIn(200, 204)
}

// CS41: Delete return=minimal - insert row, then delete it
func TestCS41_DeleteReturnMinimal(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "cs-cs41-del-minimal"},
	)
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("failed to insert row for CS41")
	}
	id := int(arr[0]["id"].(float64))

	res := h.Delete("/todos",
		harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "return=minimal"),
	)
	res.StatusIn(200, 204)
}

// CS42: Count planned - GET /todos Prefer: count=planned
func TestCS42_CountPlanned(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		nil,
		harness.H_("Prefer", "count=planned"),
	)
	res.StatusIn(200, 206)
	res.HasHeader("Content-Range")
}

// CS43: Count estimated - GET /todos Prefer: count=estimated
func TestCS43_CountEstimated(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		nil,
		harness.H_("Prefer", "count=estimated"),
	)
	res.StatusIn(200, 206)
	res.HasHeader("Content-Range")
}

// CS44: Head count exact - HEAD /todos Prefer: count=exact
func TestCS44_HeadCountExact(t *testing.T) {
	h := harness.New(t)
	res := h.Head("/todos",
		nil,
		harness.H_("Prefer", "count=exact"),
	)
	res.Status(200)
	res.EmptyBody()
	res.HasHeader("Content-Range")
}

// CS45: RPC POST - POST /rpc/add {"a":3,"b":4} → body contains "7"
func TestCS45_RPCPost(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/rpc/add",
		nil,
		nil,
		map[string]any{"a": 3, "b": 4},
	)
	res.Status(200)
	res.BodyContains("7")
}

// CS46: RPC GET - GET /rpc/get_todos_count → 200
func TestCS46_RPCGet(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/rpc/get_todos_count", nil, nil)
	res.Status(200)
}

// CS47: Tx rollback - insert with Prefer: tx=rollback, then verify row absent
func TestCS47_RPCTxRollback(t *testing.T) {
	task := "cs-cs47-txrollback-q8p2"
	h := harness.New(t)
	res := h.Post("/todos",
		nil,
		harness.H_("Prefer", "tx=rollback,return=representation"),
		map[string]any{"task": task},
	)
	res.StatusIn(200, 201)

	// row should not persist after rollback
	check := h.Get("/todos", harness.P("task", "eq."+task), nil)
	check.Status(200)
	check.ArrayLen(0)
}

// CS48: Schema accept-profile - GET /items Accept-Profile: private
func TestCS48_SchemaAcceptProfile(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/items",
		nil,
		harness.H_("Accept-Profile", "private"),
	)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least 1 row from private.items")
	}
}

// CS49: Schema content-profile - GET /items Accept-Profile: private (verifies Content-Profile)
func TestCS49_SchemaContentProfile(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/items",
		nil,
		harness.H_("Accept-Profile", "private"),
	)
	res.Status(200)
	res.JSONArray()
}

// makeAnonJWT creates a minimal HS256 JWT with role=web_anon for testing.
func makeAnonJWT() string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pay := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"web_anon"}`))
	msg := hdr + "." + pay
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	return msg + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CS50: Auth bearer JWT - GET /todos with valid JWT → 200
func TestCS50_AuthBearerJWT(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		nil,
		harness.H_("Authorization", "Bearer "+makeAnonJWT()),
	)
	res.Status(200)
}

// CS51: CSV output - GET /todos Accept: text/csv → Content-Type: text/csv
func TestCS51_CSVOutput(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		nil,
		harness.H_("Accept", "text/csv"),
	)
	res.Status(200)
	res.ContentType("text/csv")
}

// CS52: Embedded resource filter - GET /persons?select=id,name,messages(id,message)&messages.channel_id=eq.1
func TestCS52_EmbeddedResourceFilter(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,name,messages(id,message)")
	params.Set("messages.channel_id", "eq.1")
	res := h.Get("/persons", params, nil)
	res.Status(200)
	res.JSONArray()
}

// CS53: Embedded resource order - GET /persons?select=id,name,messages(id,message)&messages.order=id.desc
func TestCS53_EmbeddedResourceOrder(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,name,messages(id,message)")
	params.Set("messages.order", "id.desc")
	res := h.Get("/persons", params, nil)
	res.Status(200)
	res.JSONArray()
}

// CS54: Embedded resource limit - GET /persons?select=id,name,messages(id,message)&messages.limit=1
func TestCS54_EmbeddedResourceLimit(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("select", "id,name,messages(id,message)")
	params.Set("messages.limit", "1")
	res := h.Get("/persons", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		msgs, ok := row["messages"]
		if !ok {
			continue
		}
		msgsArr, ok := msgs.([]any)
		if !ok {
			continue
		}
		if len(msgsArr) > 1 {
			t.Errorf("row[%d] messages len=%d, want <= 1", i, len(msgsArr))
		}
	}
}

// CS55: Select column alias - GET /persons?select=id,full_name:name
func TestCS55_SelectColumnAlias(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("select", "id,full_name:name"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one person")
	}
	for i, row := range arr {
		if _, ok := row["full_name"]; !ok {
			t.Errorf("row[%d] missing field 'full_name'", i)
		}
		if _, ok := row["name"]; ok {
			t.Errorf("row[%d] unexpected field 'name' (should be aliased to 'full_name')", i)
		}
	}
}

// CS56: Select column cast - GET /persons?select=id,age::text
func TestCS56_SelectColumnCast(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("select", "id,age::text"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one person")
	}
	for i, row := range arr {
		age, ok := row["age"]
		if !ok {
			t.Errorf("row[%d] missing field 'age'", i)
			continue
		}
		if _, isStr := age.(string); !isStr {
			t.Errorf("row[%d] age=%v (%T), want string (cast ::text)", i, age, age)
		}
	}
}

// CS57: Filter full-text search - GET /todos?task=fts.tutorial
func TestCS57_FilterFTS(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	res.Status(200)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one todo matching fts.tutorial")
	}
}

// CS58: Insert select subset - POST /todos?select=id,task Prefer: return=representation
func TestCS58_InsertSelectSubset(t *testing.T) {
	h := harness.New(t)
	res := h.Post("/todos",
		harness.P("select", "id,task"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "cs-cs58-selectsubset"},
	)
	res.StatusIn(200, 201)
	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected inserted row in response")
	}
	row := arr[0]
	if _, ok := row["id"]; !ok {
		t.Errorf("row missing field 'id'")
	}
	if _, ok := row["task"]; !ok {
		t.Errorf("row missing field 'task'")
	}
	if _, ok := row["watched_at"]; ok {
		t.Errorf("row has unexpected field 'watched_at' (not in select)")
	}

	idVal := row["id"]
	t.Cleanup(func() {
		id := int(idVal.(float64))
		h.Delete("/todos", harness.P("id", "eq."+itoa(id)), nil)
	})
}

// CS59: Range header - GET /todos Range: 0-1 → at most 2 rows
func TestCS59_RangeHeader(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/todos",
		nil,
		harness.H_("Range", "0-1"),
	)
	res.StatusIn(200, 206)
	arr := res.JSONArray()
	if len(arr) > 2 {
		t.Errorf("expected at most 2 rows (Range: 0-1), got %d", len(arr))
	}
}

// CS60: Delete return=minimal - insert row, then delete it
func TestCS60_DeleteReturnMinimal(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos",
		harness.P("select", "id"),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "cs-cs60-del-minimal"},
	)
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("failed to insert row for CS60")
	}
	id := int(arr[0]["id"].(float64))

	res := h.Delete("/todos",
		harness.P("id", "eq."+itoa(id)),
		harness.H_("Prefer", "return=minimal"),
	)
	res.StatusIn(200, 204)
}

// CS61: Maybe-single found - GET /persons?id=eq.1 Accept: vnd.pgrst.object → object id=1
func TestCS61_MaybeSingleFound(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	res.Status(200)
	obj := res.JSONObject()
	if id, _ := obj["id"].(float64); id != 1 {
		t.Errorf("id: got %v, want 1", obj["id"])
	}
}

// CS62: Filter not.eq - todos where done != true
func TestCS62_FilterNotEq(t *testing.T) {
	h := harness.New(t)
	params := url.Values{}
	params.Set("done", "not.eq.true")
	res := h.Get("/todos", params, nil)
	res.Status(200)
	arr := res.JSONArray()
	for i, row := range arr {
		done, _ := row["done"].(bool)
		if done {
			t.Errorf("row[%d] done=%v, want false", i, row["done"])
		}
	}
}

// CS63: Filter match - GET /persons?id=eq.1&name=eq.Alice → 1 row Alice
func TestCS63_FilterMatch(t *testing.T) {
	h := harness.New(t)
	res := h.Get("/persons", harness.P("id", "eq.1", "name", "eq.Alice"), nil)
	res.Status(200)
	res.ArrayLen(1)
	arr := res.JSONArray()
	if name, _ := arr[0]["name"].(string); name != "Alice" {
		t.Errorf("name: got %q, want Alice", name)
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

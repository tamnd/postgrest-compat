// Package rs_test verifies wire compatibility with the Rust postgrest crate
// (crates.io: "postgrest" v1.6.0, repo: supabase-community/postgrest-rs).
//
// Rust-specific wire characteristics:
//   - ALL filter values are &str — caller serializes, no type wrapper.
//   - NO automatic Prefer headers (return, count, resolution). Caller sets them.
//   - NO .single()/.csv() methods. Caller sets Accept at client level.
//   - Raw pre-serialized JSON string body for writes.
//   - .auth(jwt) is a per-query builder method → Authorization: Bearer jwt.
//   - Global headers set at client construction via insert_header.
//   - Range/array operators named cs/cd/ov/sl/sr/nxl/nxr/adj directly.
//   - FTS: .fts(col, q) → ?col=fts.q (no config parameter).
package rs_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// RS1: Insert raw JSON body — POST /todos, no Prefer: return → 201, empty body.
//
// Rust wire:
//
//	client.from("todos").insert(r#"{"task":"rust insert"}"#).execute().await?
func TestRS1_InsertRawJSON(t *testing.T) {
	h := harness.New(t)

	res := h.Post("/todos", nil, nil, map[string]any{"task": "rust insert"})
	res.Status(201)

	// Clean up: delete the inserted row.
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.rust insert"), nil)
	})
}

// RS2: Bulk insert array — POST /todos, body is a JSON array → 201.
//
// Rust wire:
//
//	client.from("todos").insert(r#"[{"task":"a"},{"task":"b"}]"#).execute().await?
func TestRS2_InsertBulkArray(t *testing.T) {
	h := harness.New(t)

	body := []map[string]any{
		{"task": "rs-bulk-a"},
		{"task": "rs-bulk-b"},
	}
	res := h.Post("/todos", nil, nil, body)
	res.Status(201)

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "like.rs-bulk-%"), nil)
	})
}

// RS3: Upsert with manually-set Prefer: resolution=merge-duplicates → 200/201.
//
// The Rust crate uses .upsert() which sends POST with
// Prefer: resolution=merge-duplicates. The caller must set it explicitly.
//
// Rust wire (global header at construction):
//
//	Postgrest::new(url)
//	    .insert_header("Prefer", "resolution=merge-duplicates")
//	client.from("todos").upsert(r#"{"task":"upsert-rs"}"#).execute().await?
func TestRS3_UpsertWithManualPrefer(t *testing.T) {
	h := harness.New(t)

	// First insert to have something to upsert.
	h.Post("/todos", nil, nil, map[string]any{"task": "upsert-rs", "done": false})

	// Upsert via POST with resolution header set by caller.
	res := h.Post("/todos", nil,
		harness.H_("Prefer", "resolution=merge-duplicates"),
		map[string]any{"task": "upsert-rs", "done": true},
	)
	res.StatusIn(200, 201)

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.upsert-rs"), nil)
	})
}

// RS4: Update raw body — PATCH /todos?done=eq.false, body string → 204.
//
// Rust wire:
//
//	client.from("todos").eq("done","false").update(r#"{"done":true}"#).execute().await?
//
// We target a specific task to avoid mutating seed rows permanently.
func TestRS4_UpdateRawBody(t *testing.T) {
	h := harness.New(t)

	// Insert a fresh row to update.
	h.Post("/todos", nil, nil, map[string]any{"task": "rs-update-me", "done": false})

	res := h.Patch("/todos",
		harness.P("task", "eq.rs-update-me"),
		nil,
		map[string]any{"done": true},
	)
	res.Status(204)

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.rs-update-me"), nil)
	})
}

// RS5: Delete — DELETE /todos?task=eq.rs-test-row → 204.
//
// Rust wire:
//
//	client.from("todos").eq("task","rs-test-row").delete().execute().await?
func TestRS5_Delete(t *testing.T) {
	h := harness.New(t)

	// Insert a row to delete.
	h.Post("/todos", nil, nil, map[string]any{"task": "rs-test-row"})

	res := h.Delete("/todos", harness.P("task", "eq.rs-test-row"), nil)
	res.Status(204)
}

// RS6: Count via caller-set header — Prefer: count=exact → Content-Range with total.
//
// Rust wire (global header):
//
//	Postgrest::new(url).insert_header("Prefer", "count=exact")
//	client.from("todos").select("*").execute().await?
func TestRS6_CountViaCallerHeader(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", nil,
		harness.H_("Prefer", "count=exact"),
	)
	res.Status(200)
	res.HasHeader("Content-Range")

	cr := res.Header("Content-Range")
	total := harness.ContentRangeTotal(cr)
	if total < 0 {
		t.Errorf("Content-Range total should be a non-negative integer, got: %s", cr)
	}
}

// RS7: Return representation via caller-set header — Prefer: return=representation →
// body not empty on insert.
//
// Rust wire:
//
//	Postgrest::new(url).insert_header("Prefer", "return=representation")
//	client.from("todos").insert(r#"{"task":"rs-repr"}"#).execute().await?
func TestRS7_ReturnRepresentationViaCallerHeader(t *testing.T) {
	h := harness.New(t)

	res := h.Post("/todos", nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "rs-repr"},
	)
	res.StatusIn(200, 201)

	body := res.RawBody()
	if len(body) == 0 {
		t.Error("expected non-empty body when Prefer: return=representation")
	}

	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.rs-repr"), nil)
	})
}

// RS8: FTS no config — GET /todos?task=fts.tutorial → 200.
//
// Rust wire: .fts("task","tutorial") → ?task=fts.tutorial
// Unlike JS/Go, no config parameter.
func TestRS8_FTSNoConfig(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	res.Status(200)
}

// RS9: plfts — GET /todos?task=plfts.tutorial → 200.
//
// Rust wire: .plfts("task","tutorial") → ?task=plfts.tutorial
func TestRS9_PLFTS(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "plfts.tutorial"), nil)
	res.Status(200)
}

// RS10: cs (contains) — GET /todos?tags=cs.{go} → rows with go in tags.
//
// Rust wire: .cs("tags","{go}") → ?tags=cs.{go}
// Seed: todo id=1 has tags {go,sql}.
func TestRS10_CSContains(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("tags", "cs.{go}"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with tags containing 'go'")
	}
}

// RS11: cd (contained by) — GET /todos?tags=cd.{go,sql,pets,chores,home} →
// all rows whose tags are a subset of that set.
//
// Rust wire: .cd("tags","{go,sql,pets,chores,home}") → ?tags=cd.{go,sql,pets,chores,home}
// Seed: all 3 todos should qualify since their tags are subsets.
func TestRS11_CDContainedBy(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("tags", "cd.{go,sql,pets,chores,home}"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row")
	}
}

// RS12: ov (overlaps) — GET /todos?tags=ov.{go,pets} → rows with go or pets.
//
// Rust wire: .ov("tags","{go,pets}") → ?tags=ov.{go,pets}
// Seed: id=1 has {go,sql}, id=2 has {pets}.
func TestRS12_OVOverlaps(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("tags", "ov.{go,pets}"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with overlapping tags")
	}
}

// RS13: String filter values — GET /todos?id=gt.1 (string not int) → 200.
//
// Rust wire: .gt("id","1") → ?id=gt.1
// The Rust crate passes all values as &str. Server parses and casts to integer.
func TestRS13_StringFilterValues(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "gt.1"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		idRaw, ok := row["id"]
		if !ok {
			t.Errorf("row[%d] missing 'id' field", i)
			continue
		}
		idNum, ok := idRaw.(float64)
		if !ok {
			t.Errorf("row[%d] id is not a number: %T", i, idRaw)
			continue
		}
		if idNum <= 1 {
			t.Errorf("row[%d] id=%v should be > 1", i, idNum)
		}
	}
}

// RS14: Per-query auth — GET /todos with Authorization: Bearer <token>.
//
// Rust wire: client.from("todos").auth("some-jwt").select("*").execute().await?
// .auth(jwt) sets Authorization: Bearer jwt for that request only.
// We use a dummy token and just verify the server responds (not 500).
func TestRS14_PerQueryAuth(t *testing.T) {
	h := harness.New(t)

	// Anonymous request (no auth) should work since web_anon has select grants.
	res := h.Get("/todos", nil, nil)
	res.Status(200)

	// Request with a bearer token header (simulating .auth(jwt) call).
	// We use an invalid token — server may return 401/403 or 200 depending on anon config.
	resAuth := h.Get("/todos", nil,
		harness.H_("Authorization", "Bearer invalid-jwt-token"),
	)
	// The server should not 500; 200, 401, or 403 are all valid.
	code := resAuth.Header("") // just force eval; check via StatusIn
	_ = code
	resAuth.StatusIn(200, 401, 403)
}

// RS15: Nested filter with not — GET /todos?done=not.eq.true → undone todos.
//
// Rust wire: .not("done","eq","true") → ?done=not.eq.true
func TestRS15_NotFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("done", "not.eq.true"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		done, ok := row["done"]
		if !ok {
			continue
		}
		if b, ok := done.(bool); ok && b {
			t.Errorf("row[%d] has done=true but not.eq.true filter should exclude it", i)
		}
	}
}

// RS16: or filter — GET /todos?or=(done.eq.true,id.eq.1) → rows where done=true or id=1.
//
// Rust wire: .or("done.eq.true,id.eq.1") → ?or=(done.eq.true,id.eq.1)
func TestRS16_OrFilter(t *testing.T) {
	h := harness.New(t)

	// url.Values.Set will encode the parentheses; PostgREST expects them in the value.
	params := url.Values{}
	params.Set("or", "(done.eq.true,id.eq.1)")

	res := h.Get("/todos", params, nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row for or=(done.eq.true,id.eq.1)")
	}

	for i, row := range arr {
		id, hasID := row["id"]
		done, hasDone := row["done"]
		if !hasID || !hasDone {
			continue
		}
		idNum, _ := id.(float64)
		doneBool, _ := done.(bool)
		if idNum != 1 && !doneBool {
			t.Errorf("row[%d] does not satisfy or condition: id=%v done=%v", i, id, done)
		}
	}
}

// RS17: adj range — GET /todos?id=adj.(0,2) → id adjacent to range (0,2).
//
// Rust wire: .adj("id","(0,2)") → ?id=adj.(0,2)
// Adjacent means the range touches but does not overlap. For integer id,
// values adjacent to (0,2) would be id=2 (right adjacency) or id≤0 (left),
// but since ids start at 1, this is a correctness test that server handles
// the operator without error.
func TestRS17_AdjRange(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "adj.(0,2)"), nil)
	// Server must handle the adj operator — 200 or 200 with empty array are both valid.
	res.Status(200)
}

// RS_Eq: Basic eq filter — string value → 200 with matching rows only.
//
// Rust wire: .eq("task","finish tutorial") → ?task=eq.finish tutorial
func TestRS_EqFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "eq.finish tutorial"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) != 1 {
		t.Errorf("expected exactly 1 row, got %d", len(arr))
	}
}

// RS_Neq: neq filter → rows not matching.
//
// Rust wire: .neq("done","true") → ?done=neq.true
func TestRS_NeqFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("done", "neq.true"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		done, ok := row["done"]
		if !ok {
			continue
		}
		if b, ok := done.(bool); ok && b {
			t.Errorf("row[%d] has done=true, but neq.true should exclude it", i)
		}
	}
}

// RS_Like: like filter — pattern match.
//
// Rust wire: .like("task","finish%") → ?task=like.finish%
func TestRS_LikeFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "like.finish%"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		task, ok := row["task"].(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(task, "finish") {
			t.Errorf("row[%d] task %q does not match like.finish%%", i, task)
		}
	}
}

// RS_ILike: ilike filter — case-insensitive pattern match.
//
// Rust wire: .ilike("task","FINISH%") → ?task=ilike.FINISH%
func TestRS_ILikeFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "ilike.FINISH%"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		task, ok := row["task"].(string)
		if !ok {
			continue
		}
		if !strings.EqualFold(task[:min(len(task), 6)], "finish") {
			t.Errorf("row[%d] task %q does not match ilike.FINISH%%", i, task)
		}
	}
}

// RS_Is: is filter — null check.
//
// Rust wire: .is("due","null") → ?due=is.null
// Seed: todo id=2 has due=null.
func TestRS_IsNull(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("due", "is.null"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with due=null")
	}
	for i, row := range arr {
		if row["due"] != nil {
			t.Errorf("row[%d] due=%v, expected null", i, row["due"])
		}
	}
}

// RS_In: in_ filter — value in set.
//
// Rust wire: .in_("id","(1,2)") → ?id=in.(1,2)
func TestRS_InFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "in.(1,2)"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row")
	}
	for i, row := range arr {
		idRaw, ok := row["id"].(float64)
		if !ok {
			t.Errorf("row[%d] id not a number", i)
			continue
		}
		if idRaw != 1 && idRaw != 2 {
			t.Errorf("row[%d] id=%v not in (1,2)", i, idRaw)
		}
	}
}

// RS_Lte: lte filter — less than or equal (string value).
//
// Rust wire: .lte("id","2") → ?id=lte.2
func TestRS_LteFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "lte.2"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		idRaw, ok := row["id"].(float64)
		if !ok {
			continue
		}
		if idRaw > 2 {
			t.Errorf("row[%d] id=%v > 2, lte.2 should exclude it", i, idRaw)
		}
	}
}

// RS_Select: select columns — GET /todos?select=id,task → only those fields.
//
// Rust wire: client.from("todos").select("id,task").execute().await?
// → GET /todos?select=id,task
func TestRS_SelectColumns(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("select", "id,task"), nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row")
	}
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing field 'id'", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing field 'task'", i)
		}
		// 'done' should not be present since we selected only id,task.
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] has 'done' field but it was not selected", i)
		}
	}
}

// RS_OrderLimit: order + limit — GET /todos?order=id.desc&limit=2
//
// Rust wire:
//
//	client.from("todos").select("*").order("id.desc").limit(2).execute().await?
//	→ GET /todos?select=*&order=id.desc&limit=2
func TestRS_OrderLimit(t *testing.T) {
	h := harness.New(t)

	params := harness.P("order", "id.desc", "limit", "2")
	res := h.Get("/todos", params, nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) > 2 {
		t.Errorf("expected at most 2 rows, got %d", len(arr))
	}
	if len(arr) > 1 {
		id0, _ := arr[0]["id"].(float64)
		id1, _ := arr[1]["id"].(float64)
		if id0 < id1 {
			t.Errorf("rows not in descending order: id[0]=%v id[1]=%v", id0, id1)
		}
	}
}

// RS_Offset: offset parameter — GET /todos?offset=1
//
// Rust wire: client.from("todos").select("*").range(1,2).execute().await?
// → GET /todos?offset=1&limit=2 (or just offset)
func TestRS_Offset(t *testing.T) {
	h := harness.New(t)

	// First get all rows to know total.
	allRes := h.Get("/todos", nil, nil)
	allRes.Status(200)
	all := allRes.JSONArray()

	params := harness.P("offset", "1")
	res := h.Get("/todos", params, nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) != len(all)-1 {
		t.Errorf("offset=1 should return %d rows, got %d", len(all)-1, len(arr))
	}
}

// RS_RPC: RPC call — POST /rpc/add → result.
//
// Rust wire: client.rpc("add", r#"{"a":1,"b":2}"#).execute().await?
// → POST /rpc/add, body: {"a":1,"b":2}
func TestRS_RPC(t *testing.T) {
	h := harness.New(t)

	res := h.Post("/rpc/add", nil, nil, map[string]any{"a": 1, "b": 2})
	// 200 or 204 depending on function definition; just verify it's not a 4xx/5xx error.
	res.StatusIn(200, 204)
}

// RS_SchemaSwitch: schema-switching via Accept-Profile header → /items from private schema.
//
// The Rust caller sets Accept-Profile as a global header:
//
//	Postgrest::new(url).insert_header("Accept-Profile", "private")
func TestRS_SchemaSwitch(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/items", nil,
		harness.H_("Accept-Profile", "private"),
	)
	res.Status(200)

	arr := res.JSONArray()
	// Seed has 2 private items.
	if len(arr) == 0 {
		t.Error("expected at least one item from private schema")
	}
}

// RS_SingleObject: Accept: application/vnd.pgrst.object+json → single object.
//
// Rust caller sets Accept via insert_header at client level.
// Rust wire:
//
//	Postgrest::new(url).insert_header("Accept","application/vnd.pgrst.object+json")
//	client.from("todos").eq("id","1").select("*").execute().await?
func TestRS_SingleObject(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	res.StatusIn(200, 406)

	if res.Header("") == "" { // force evaluation; actual check below
	}
	// If 200, body should be an object not an array.
	if res.Header("Content-Type") != "" {
		body := res.RawBody()
		if len(body) > 0 && body[0] == '[' {
			t.Errorf("expected JSON object, got array: %s", body)
		}
	}
}

// RS_CSVAccept: Accept: text/csv → CSV response.
//
// Rust caller sets Accept via insert_header.
// Rust wire:
//
//	Postgrest::new(url).insert_header("Accept","text/csv")
//	client.from("todos").select("*").execute().await?
func TestRS_CSVAccept(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", nil,
		harness.H_("Accept", "text/csv"),
	)
	res.Status(200)
	res.ContentType("text/csv")
}

// RS_GlobalAuthHeader: Authorization set via insert_header at client level.
//
// Rust wire:
//
//	Postgrest::new(url).insert_header("Authorization","Bearer <token>")
//	client.from("todos").select("*").execute().await?
//
// Here we test with no actual token (anonymous access should still work).
func TestRS_GlobalAuthHeader(t *testing.T) {
	h := harness.New(t)

	// No auth header — anonymous access.
	res := h.Get("/todos", nil, nil)
	res.Status(200)
}

// RS_FilterMatch: multiple eq filters (match) — simulated multi-param.
//
// Rust wire: .match(r#"{"done":false,"id":"1"}"#) → ?done=eq.false&id=eq.1
// We use url.Values directly for multiple params.
func TestRS_FilterMatch(t *testing.T) {
	h := harness.New(t)

	params := url.Values{}
	params.Set("done", "eq.false")
	params.Set("id", "eq.1")

	res := h.Get("/todos", params, nil)
	res.Status(200)

	arr := res.JSONArray()
	if len(arr) != 1 {
		t.Errorf("expected 1 row, got %d", len(arr))
	}
}

// RS_GtFilter: gt filter — GET /todos?id=gt.1 (string value, not int).
//
// Rust wire: .gt("id","1") → ?id=gt.1
func TestRS_GtFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "gt.1"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		idRaw, _ := row["id"].(float64)
		if idRaw <= 1 {
			t.Errorf("row[%d] id=%v should be > 1", i, idRaw)
		}
	}
}

// RS_GteFilter: gte filter — GET /todos?id=gte.2
//
// Rust wire: .gte("id","2") → ?id=gte.2
func TestRS_GteFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "gte.2"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		idRaw, _ := row["id"].(float64)
		if idRaw < 2 {
			t.Errorf("row[%d] id=%v should be >= 2", i, idRaw)
		}
	}
}

// RS_LtFilter: lt filter — GET /todos?id=lt.3
//
// Rust wire: .lt("id","3") → ?id=lt.3
func TestRS_LtFilter(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("id", "lt.3"), nil)
	res.Status(200)

	arr := res.JSONArray()
	for i, row := range arr {
		idRaw, _ := row["id"].(float64)
		if idRaw >= 3 {
			t.Errorf("row[%d] id=%v should be < 3", i, idRaw)
		}
	}
}

// RS_WFts: wfts operator — websearch FTS.
//
// Rust wire: .wfts("task","tutorial") → ?task=wfts.tutorial
func TestRS_WFts(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "wfts.tutorial"), nil)
	res.Status(200)
}

// RS_PhFts: phfts operator — phrase FTS.
//
// Rust wire: .phfts("task","finish tutorial") → ?task=phfts.finish tutorial
func TestRS_PhFts(t *testing.T) {
	h := harness.New(t)

	res := h.Get("/todos", harness.P("task", "phfts.finish tutorial"), nil)
	res.Status(200)
}

// min is a helper for integer minimum used in string slicing.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

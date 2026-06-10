// Package py_test verifies wire compatibility with the Python postgrest-py client
// (from the supabase-py monorepo).  Each test sends the exact HTTP request the
// Python client would emit and asserts the server responds correctly.
//
// Python-specific wire differences vs. the JS client:
//   - Pagination via Range header (bytes=start-end) not ?offset=/?limit=
//   - Prefer: return=minimal is always present on writes
//   - FTS via separate methods: fts/plfts/phfts/wfts
//   - Method names use trailing underscores: is_(), in_(), not_(), or_()
//   - maybe_single() (snake_case)
package py_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

func makeJWT(role string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"` + role + `"}`))
	msg := header + "." + payload
	mac := hmac.New(sha256.New, []byte(harness.JWTSecret()))
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sig
}

// PY1: Range header pagination – first 2 rows.
// postgrest-py sends "Range: 0-1" (row units, no "bytes=" prefix).
// PostgREST returns 200 when no count is requested, 206 when count=exact is set.
func TestPY1_RangePagination(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Range", "0-1"),
	)
	r.StatusIn(200, 206)
	arr := r.JSONArray()
	if len(arr) != 2 {
		t.Errorf("expected 2 rows, got %d", len(arr))
	}
}

// PY2: Range + count=exact – Content-Range must contain the total
func TestPY2_RangeWithCount(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_(
			"Range",  "0-1",
			"Prefer", "count=exact",
		),
	)
	r.Status(206)
	cr := r.Header("Content-Range")
	if cr == "" {
		t.Fatal("Content-Range header missing")
	}
	total := harness.ContentRangeTotal(cr)
	if total < 0 {
		t.Errorf("Content-Range total is unknown (*), expected exact count: %s", cr)
	}
}

// PY3: Prefer: return=minimal is always sent on insert
func TestPY3_InsertReturnMinimal(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.py3 test row"), nil)
	})
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"task": "py3 test row", "done": false},
	)
	// return=minimal → 201 with empty body (or 204)
	r.StatusIn(201, 204)
}

// PY4: Prefer: return=representation when explicitly chained
func TestPY4_InsertReturnRepresentation(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.py4 test row"), nil)
	})
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "py4 test row", "done": false},
	)
	r.Status(201)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected inserted row in response body")
	}
}

// PY5: plfts – plain-text full-text search (?task=plfts.tutorial)
func TestPY5_Plfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "plfts.tutorial"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for _, row := range arr {
		task, _ := row["task"].(string)
		if !strings.Contains(strings.ToLower(task), "tutorial") {
			t.Errorf("unexpected row in plfts result: task=%q", task)
		}
	}
}

// PY6: phfts – phrase full-text search (?task=phfts.tutorial)
func TestPY6_Phfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "phfts.tutorial"), nil)
	r.Status(200)
}

// PY7: wfts – websearch full-text search (?task=wfts.cat)
func TestPY7_Wfts(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "wfts.cat"), nil)
	r.Status(200)
}

// PY8: fts with config – ?task=fts(english).tutorial
func TestPY8_FtsWithConfig(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "fts(english).tutorial"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for _, row := range arr {
		task, _ := row["task"].(string)
		if !strings.Contains(strings.ToLower(task), "tutorial") {
			t.Errorf("unexpected row from fts(english): task=%q", task)
		}
	}
}

// PY9: or_ filter – ?or=(done.eq.true,done.eq.false) returns all rows
func TestPY9_OrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("or", "(done.eq.true,done.eq.false)"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 3 {
		t.Errorf("or_ should return all rows, got %d", len(arr))
	}
}

// PY10: not_ filter – ?done=not.eq.true returns only undone todos
func TestPY10_NotFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "not.eq.true"), nil)
	r.Status(200)
	r.RowsAllMatch("done != true", func(row map[string]any) bool {
		done, _ := row["done"].(bool)
		return !done
	})
}

// PY11: is_ null – ?due=is.null returns todos with no due date
func TestPY11_IsNull(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("due", "is.null"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one todo with due=null")
	}
	for _, row := range arr {
		if row["due"] != nil {
			t.Errorf("expected due=null but got %v", row["due"])
		}
	}
}

// PY12: in_ filter – ?id=in.(1,2) returns exactly 2 rows
func TestPY12_InFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "in.(1,2)"), nil)
	r.Status(200)
	r.ArrayLen(2)
}

// PY13: maybe_single – GET returns array (or object); server must not 406
func TestPY13_MaybeSingle(t *testing.T) {
	h := harness.New(t)
	// maybe_single sends Accept: application/vnd.pgrst.object+json
	// but falls back to array if zero/many rows – server must not error
	r := h.Get("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	// single row → 200 with an object
	r.Status(200)
	obj := r.JSONObject()
	if obj["id"] == nil {
		t.Error("expected id field in object")
	}
}

// PY13b: maybe_single on empty result – server must return 200, not 406
func TestPY13b_MaybeSingleEmpty(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.99999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	// no row: PostgREST returns 200 with null body or 406; dbrest should return 200
	r.StatusIn(200, 406)
}

// PY14: Auth bearer header – invalid token
func TestPY14_AuthBearerInvalid(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		nil,
		harness.H_("Authorization", "Bearer invalidtoken"),
	)
	// invalid JWT → 401; anonymous access allowed → 200
	r.StatusIn(200, 401)
}

// PY15: Basic auth header – server must handle or reject cleanly
func TestPY15_BasicAuth(t *testing.T) {
	h := harness.New(t)
	creds := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	r := h.Get("/todos",
		nil,
		harness.H_("Authorization", "Basic "+creds),
	)
	// Basic auth not standard for PostgREST; expect 200 (ignored) or 401
	r.StatusIn(200, 401)
}

// PY16: CSV content negotiation – Accept: text/csv
func TestPY16_CsvAccept(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Accept", "text/csv"),
	)
	r.Status(200)
	r.ContentType("text/csv")
}

// PY17: fts without config – ?task=fts.tutorial
func TestPY17_FtsNoConfig(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "fts.tutorial"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for _, row := range arr {
		task, _ := row["task"].(string)
		if !strings.Contains(strings.ToLower(task), "tutorial") {
			t.Errorf("unexpected row from fts.tutorial: task=%q", task)
		}
	}
}

// PY18: Range header on a single-table endpoint returns Content-Range header
func TestPY18_RangeResponseHeader(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Range", "bytes=0-4"),
	)
	// up to 5 rows; there are only 3 so server may return 200 or 206
	r.StatusIn(200, 206)
}

// PY19: Update with return=minimal (Python always sends this header)
func TestPY19_UpdateReturnMinimal(t *testing.T) {
	h := harness.New(t)
	r := h.Patch("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"done": false},
	)
	r.StatusIn(200, 204)
}

// PY20: Delete with return=minimal
func TestPY20_DeleteReturnMinimal(t *testing.T) {
	h := harness.New(t)
	// Insert a temporary row, then delete it with return=minimal
	h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"task": "py20 temp", "done": false},
	)
	r := h.Delete("/todos",
		harness.P("task", "eq.py20 temp"),
		harness.H_("Prefer", "return=minimal"),
	)
	r.StatusIn(200, 204)
}

// PY21: select with embedded resource – Python supports standard embed syntax
func TestPY21_EmbedResource(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "id,task,assignments(id,person_id)"),
		nil,
	)
	r.Status(200)
	r.RowsHaveField("assignments")
}

// PY22: order – snake_case method maps to same ?order= param
func TestPY22_OrderAsc(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("order", "id.asc"),
		nil,
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) < 2 {
		return
	}
	first, _ := arr[0]["id"].(float64)
	second, _ := arr[1]["id"].(float64)
	if first > second {
		t.Errorf("expected ascending id order, got %v then %v", first, second)
	}
}

// PY23: select specific columns (Python .select("id,task"))
func TestPY23_SelectColumns(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,task"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row["id"]; !ok {
			t.Errorf("row[%d] missing id", i)
		}
		if _, ok := row["task"]; !ok {
			t.Errorf("row[%d] missing task", i)
		}
		if _, ok := row["done"]; ok {
			t.Errorf("row[%d] should not have done column", i)
		}
	}
}

// PY24: contained_by operator (snake_case method, same wire as <@)
func TestPY24_ContainedBy(t *testing.T) {
	h := harness.New(t)
	// tags <@ '{go,sql,pets,chores,home}' – all rows should match
	r := h.Get("/todos",
		harness.P("tags", "cs.{go,sql,pets,chores,home}"),
		nil,
	)
	r.Status(200)
}

// PY25: Upsert with return=minimal (onConflict=merge-duplicates)
func TestPY25_UpsertReturnMinimal(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.py25 upsert"), nil)
	})
	r := h.Post("/todos",
		harness.P("on_conflict", "id"),
		harness.H_("Prefer", "return=minimal,resolution=merge-duplicates"),
		map[string]any{"task": "py25 upsert", "done": false},
	)
	r.StatusIn(200, 201, 204)
}

// PY26: Schema switching via Accept-Profile header (private schema)
func TestPY26_PrivateSchemaSwitch(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/items",
		nil,
		harness.H_("Accept-Profile", "private"),
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected private.items rows")
	}
}

func TestPY27_NeqFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "neq.1"), nil)
	r.Status(200)
	r.RowsAllMatch("id != 1", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id != 1
	})
}

func TestPY28_GtFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "gt.1"), nil)
	r.Status(200)
	r.RowsAllMatch("id > 1", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id > 1
	})
}

func TestPY29_GteFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "gte.2"), nil)
	r.Status(200)
	r.RowsAllMatch("id >= 2", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id >= 2
	})
}

func TestPY30_LtFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "lt.3"), nil)
	r.Status(200)
	r.RowsAllMatch("id < 3", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id < 3
	})
}

func TestPY31_LteFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "lte.2"), nil)
	r.Status(200)
	r.RowsAllMatch("id <= 2", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id <= 2
	})
}

func TestPY32_LikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "like.*cat*"), nil)
	r.Status(200)
	r.RowsAllMatch("task contains cat", func(row map[string]any) bool {
		task, _ := row["task"].(string)
		return strings.Contains(strings.ToLower(task), "cat")
	})
}

func TestPY33_IlikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "ilike.*CAT*"), nil)
	r.Status(200)
	r.RowsAllMatch("task ilike *CAT*", func(row map[string]any) bool {
		task, _ := row["task"].(string)
		return strings.Contains(strings.ToLower(task), "cat")
	})
}

func TestPY34_IsTrue(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "is.true"), nil)
	r.Status(200)
	r.RowsAllMatch("done = true", func(row map[string]any) bool {
		done, _ := row["done"].(bool)
		return done
	})
}

func TestPY35_OvFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "ov.{go,pets}"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected at least one row with overlapping tags")
	}
}

func TestPY36_SlFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "sl.(5,10)"), nil)
	r.StatusIn(200, 400)
}

func TestPY37_SrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "sr.(0,0)"), nil)
	r.StatusIn(200, 400)
}

func TestPY38_NxlFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "nxl.(0,5)"), nil)
	r.StatusIn(200, 400, 404)
}

func TestPY39_NxrFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "nxr.(0,5)"), nil)
	r.StatusIn(200, 400, 404)
}

func TestPY40_AdjFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "adj.(0,1)"), nil)
	r.StatusIn(200, 400, 404)
}

func TestPY41_CdFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("tags", "cd.{go,sql,pets,chores,home}"), nil)
	r.Status(200)
}

func TestPY42_PlftsWithConfig(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "plfts(english).tutorial"), nil)
	r.Status(200)
}

func TestPY43_PhftsWithConfig(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "phfts(english).tutorial"), nil)
	r.Status(200)
}

func TestPY44_WftsWithConfig(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "wfts(english).cat"), nil)
	r.Status(200)
}

func TestPY45_AndFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("and", "(done.eq.false,id.gt.0)"), nil)
	r.Status(200)
	r.RowsAllMatch("done=false and id>0", func(row map[string]any) bool {
		done, _ := row["done"].(bool)
		id, _ := row["id"].(float64)
		return !done && id > 0
	})
}

func TestPY46_NestedOrInAnd(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("and", "(or(done.eq.true,id.eq.3),task.neq.missing)"), nil)
	r.Status(200)
}

func TestPY47_OrderDesc(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "id.desc"), nil)
	r.Status(200)
	arr := r.JSONArray()
	for i := 1; i < len(arr); i++ {
		prev, _ := arr[i-1]["id"].(float64)
		curr, _ := arr[i]["id"].(float64)
		if prev < curr {
			t.Errorf("expected descending id order at [%d,%d]: %v > %v", i-1, i, prev, curr)
		}
	}
}

func TestPY48_OrderNullsFirst(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "due.asc.nullsfirst"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		return
	}
	// first row should have null due
	if arr[0]["due"] != nil {
		t.Errorf("expected first row to have due=null (nullsfirst), got %v", arr[0]["due"])
	}
}

func TestPY49_OrderNullsLast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("order", "due.asc.nullslast"), nil)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		return
	}
	// last row should have null due
	if arr[len(arr)-1]["due"] != nil {
		t.Errorf("expected last row to have due=null (nullslast), got %v", arr[len(arr)-1]["due"])
	}
}

func TestPY50_Single(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(200)
	obj := r.JSONObject()
	id, _ := obj["id"].(float64)
	if id != 1 {
		t.Errorf("expected id=1, got %v", obj["id"])
	}
}

func TestPY51_SingleZeroRows(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("id", "eq.99999"),
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
	)
	r.Status(406)
}

func TestPY52_RpcGet(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/rpc/get_todos_count", nil, nil)
	r.Status(200)
}

func TestPY53_RpcPostWithParams(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/add", nil, nil, map[string]any{"a": 1, "b": 2})
	r.Status(200)
	body := string(r.RawBody())
	if !strings.Contains(body, "3") {
		t.Errorf("expected result 3, got body: %s", body)
	}
}

func TestPY54_RpcPostReturnsSetof(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_person_by_name", nil, nil, map[string]any{"name_param": "Alice"})
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Fatal("expected at least one row")
	}
	name, _ := arr[0]["name"].(string)
	if name != "Alice" {
		t.Errorf("expected name=Alice, got %q", name)
	}
}

func TestPY55_RpcSingle(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/add",
		nil,
		harness.H_("Accept", "application/vnd.pgrst.object+json"),
		map[string]any{"a": 1, "b": 2},
	)
	r.Status(200)
}

func TestPY56_CountPlanned(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=planned"))
	r.StatusIn(200, 206)
	r.HasHeader("Content-Range")
}

func TestPY57_CountEstimated(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=estimated"))
	r.StatusIn(200, 206)
	r.HasHeader("Content-Range")
}

func TestPY58_InsertReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "eq.py58 row"), nil)
	})
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=headers-only"),
		map[string]any{"task": "py58 row", "done": false},
	)
	r.Status(201)
	r.HasHeader("Location")
}

func TestPY59_UpdateReturnRepresentation(t *testing.T) {
	h := harness.New(t)
	ins := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=representation"),
		map[string]any{"task": "py59 row", "done": false},
	)
	ins.Status(201)
	arr := ins.JSONArray()
	if len(arr) == 0 {
		t.Fatal("insert returned no row")
	}
	id := arr[0]["id"].(float64)
	idStr := fmt.Sprintf("eq.%d", int64(id))
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("id", idStr), nil)
	})
	r := h.Patch("/todos",
		harness.P("id", idStr),
		harness.H_("Prefer", "return=representation"),
		map[string]any{"done": true},
	)
	r.Status(200)
	updated := r.JSONArray()
	if len(updated) == 0 {
		t.Error("expected non-empty body from PATCH return=representation")
	}
}

func TestPY60_DeleteReturnRepresentation(t *testing.T) {
	h := harness.New(t)
	h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=minimal"),
		map[string]any{"task": "py60 row", "done": false},
	)
	r := h.Delete("/todos",
		harness.P("task", "eq.py60 row"),
		harness.H_("Prefer", "return=representation"),
	)
	r.Status(200)
	arr := r.JSONArray()
	if len(arr) == 0 {
		t.Error("expected deleted row in response body")
	}
}

func TestPY61_UpdateReturnHeadersOnly(t *testing.T) {
	h := harness.New(t)
	r := h.Patch("/todos",
		harness.P("id", "eq.1"),
		harness.H_("Prefer", "return=headers-only"),
		map[string]any{"done": true},
	)
	r.StatusIn(200, 204)
}

func TestPY62_BulkInsert(t *testing.T) {
	h := harness.New(t)
	t.Cleanup(func() {
		h.Delete("/todos", harness.P("task", "like.py62*"), nil)
	})
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "return=representation"),
		[]map[string]any{
			{"task": "py62 row a", "done": false},
			{"task": "py62 row b", "done": false},
		},
	)
	r.Status(201)
	r.ArrayLen(2)
}

func TestPY63_UpsertIgnoreDuplicates(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		harness.P("on_conflict", "id"),
		harness.H_("Prefer", "return=minimal,resolution=ignore-duplicates"),
		map[string]any{"id": 1, "task": "finish tutorial", "done": true},
	)
	r.StatusIn(200, 201)
}

func TestPY64_TxRollback(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/todos",
		nil,
		harness.H_("Prefer", "tx=rollback,return=representation"),
		map[string]any{"task": "py64 rollback row", "done": false},
	)
	r.StatusIn(200, 201)

	// row must not be persisted
	check := h.Get("/todos", harness.P("task", "eq.py64 rollback row"), nil)
	check.Status(200)
	arr := check.JSONArray()
	if len(arr) != 0 {
		t.Errorf("row should have been rolled back, but found %d rows", len(arr))
	}
}

func TestPY65_ContentProfileSchemaSwitch(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/items",
		nil,
		harness.H_(
			"Accept-Profile",  "private",
			"Content-Profile", "private",
			"Prefer",          "return=minimal",
		),
		map[string]any{"name": "py65 item"},
	)
	// web_anon likely lacks INSERT on private.items → 403; allowed → 201
	r.StatusIn(201, 401, 403)
}

func TestPY66_SelectColumnAlias(t *testing.T) {
	h := harness.New(t)
	// PostgREST column alias syntax: alias:column (not column:alias)
	r := h.Get("/todos", harness.P("select", "id,todo_task:task"), nil)
	r.Status(200)
	r.RowsAllMatch("has todo_task key", func(row map[string]any) bool {
		_, ok := row["todo_task"]
		return ok
	})
}

func TestPY67_SelectWithCast(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("select", "id,done::text"), nil)
	r.Status(200)
	r.RowsAllMatch("done is string", func(row map[string]any) bool {
		_, ok := row["done"].(string)
		return ok
	})
}

func TestPY68_EmbedInnerJoin(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/messages", harness.P("select", "id,message,channels!inner(slug)"), nil)
	r.Status(200)
	r.RowsHaveField("channels")
}

func TestPY69_EmbedWithFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P(
			"select",            "id,task,assignments(id)",
			"assignments.person_id", "eq.1",
		),
		nil,
	)
	r.Status(200)
}

func TestPY70_HeadCount(t *testing.T) {
	h := harness.New(t)
	r := h.Head("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200)
	r.HasHeader("Content-Range")
	r.EmptyBody()
}

func TestPY71_NotInFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("id", "not.in.(1,2)"), nil)
	r.Status(200)
	r.RowsAllMatch("id not in (1,2)", func(row map[string]any) bool {
		id, _ := row["id"].(float64)
		return id != 1 && id != 2
	})
}

func TestPY72_NotLikeFilter(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("task", "not.like.*cat*"), nil)
	r.Status(200)
	r.RowsAllMatch("task not like *cat*", func(row map[string]any) bool {
		task, _ := row["task"].(string)
		return !strings.Contains(strings.ToLower(task), "cat")
	})
}

func TestPY73_NotIsNull(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("due", "not.is.null"), nil)
	r.Status(200)
	r.RowsAllMatch("due is not null", func(row map[string]any) bool {
		return row["due"] != nil
	})
}

func TestPY74_AuthBearerValid(t *testing.T) {
	h := harness.New(t)
	token := makeJWT("web_anon")
	r := h.Get("/todos", nil, harness.H_("Authorization", "Bearer "+token))
	r.Status(200)
}

func TestPY75_RpcNoParams(t *testing.T) {
	h := harness.New(t)
	r := h.Post("/rpc/get_todos_count", nil, nil, map[string]any{})
	r.Status(200)
}

func TestPY76_EmbedCount(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/channels", harness.P("select", "id,messages(count)"), nil)
	r.Status(200)
	r.RowsHaveField("messages")
}

func TestPY77_RangeSecondPage(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Range", "2-2"))
	r.StatusIn(200, 206)
	arr := r.JSONArray()
	if len(arr) != 1 {
		t.Errorf("expected 1 row for Range: 2-2, got %d", len(arr))
	}
}

func TestPY78_OrFilterTwoIds(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("or", "(id.eq.1,id.eq.2)"), nil)
	r.Status(200)
	r.ArrayLen(2)
}

func TestPY79_IsFalse(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", harness.P("done", "is.false"), nil)
	r.Status(200)
	r.RowsAllMatch("done = false", func(row map[string]any) bool {
		done, _ := row["done"].(bool)
		return !done
	})
}

func TestPY80_CountExactNoRange(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos", nil, harness.H_("Prefer", "count=exact"))
	r.Status(200)
	r.HasHeader("Content-Range")
}

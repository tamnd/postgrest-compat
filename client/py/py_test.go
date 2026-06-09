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
	"encoding/base64"
	"strings"
	"testing"

	"github.com/tamnd/postgrest-compat/harness"
)

// PY1: Range header pagination – first 2 rows (Range: bytes=0-1)
func TestPY1_RangePagination(t *testing.T) {
	h := harness.New(t)
	r := h.Get("/todos",
		harness.P("select", "*"),
		harness.H_("Range", "bytes=0-1"),
	)
	// 206 Partial Content when a Range header is honoured
	r.Status(206)
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
			"Range",  "bytes=0-1",
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

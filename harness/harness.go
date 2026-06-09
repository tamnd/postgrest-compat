// Package harness provides HTTP test helpers for postgrest-compat.
// Tests read POSTGREST_URL from the environment (default: http://localhost:3000).
package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

// ServerURL returns the base URL of the server under test.
// Override with POSTGREST_URL env var.
func ServerURL() string {
	if v := os.Getenv("POSTGREST_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:3000"
}

// JWTSecret returns the JWT secret used by the test stack.
func JWTSecret() string {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return v
	}
	return "reallyreallyreallyreallyverysafe"
}

// H is a test helper bound to a *testing.T.
type H struct {
	t      *testing.T
	base   string
	client *http.Client
}

// New creates a new H for the given test.
func New(t *testing.T) *H {
	t.Helper()
	return &H{t: t, base: ServerURL(), client: &http.Client{}}
}

// Req sends a request and returns a Result.
func (h *H) Req(method, path string, params url.Values, headers map[string]string, body any) *Result {
	h.t.Helper()
	u := h.base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("do request %s %s: %v", method, u, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatalf("read body: %v", err)
	}
	return &Result{t: h.t, resp: resp, body: b}
}

// Get sends GET /<path>?<params> with optional headers.
func (h *H) Get(path string, params url.Values, headers map[string]string) *Result {
	return h.Req(http.MethodGet, path, params, headers, nil)
}

// Head sends HEAD /<path>?<params>.
func (h *H) Head(path string, params url.Values, headers map[string]string) *Result {
	return h.Req(http.MethodHead, path, params, headers, nil)
}

// Post sends POST /<path> with JSON body.
func (h *H) Post(path string, params url.Values, headers map[string]string, body any) *Result {
	return h.Req(http.MethodPost, path, params, headers, body)
}

// Patch sends PATCH /<path>?<params> with JSON body.
func (h *H) Patch(path string, params url.Values, headers map[string]string, body any) *Result {
	return h.Req(http.MethodPatch, path, params, headers, body)
}

// Delete sends DELETE /<path>?<params>.
func (h *H) Delete(path string, params url.Values, headers map[string]string) *Result {
	return h.Req(http.MethodDelete, path, params, headers, nil)
}

// Result holds the server response.
type Result struct {
	t    *testing.T
	resp *http.Response
	body []byte
}

// Status asserts the HTTP status code and returns r for chaining.
func (r *Result) Status(want int) *Result {
	r.t.Helper()
	if r.resp.StatusCode != want {
		r.t.Errorf("HTTP status: got %d, want %d\nbody: %s", r.resp.StatusCode, want, r.body)
	}
	return r
}

// StatusIn asserts the HTTP status is one of the given values.
func (r *Result) StatusIn(codes ...int) *Result {
	r.t.Helper()
	for _, c := range codes {
		if r.resp.StatusCode == c {
			return r
		}
	}
	r.t.Errorf("HTTP status: got %d, want one of %v\nbody: %s", r.resp.StatusCode, codes, r.body)
	return r
}

// JSONArray asserts the body is a JSON array and returns it.
func (r *Result) JSONArray() []map[string]any {
	r.t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		r.t.Fatalf("body is not a JSON array: %v\nbody: %s", err, r.body)
	}
	return out
}

// JSONObject asserts the body is a JSON object and returns it.
func (r *Result) JSONObject() map[string]any {
	r.t.Helper()
	var out map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		r.t.Fatalf("body is not a JSON object: %v\nbody: %s", err, r.body)
	}
	return out
}

// EmptyBody asserts the body is empty.
func (r *Result) EmptyBody() *Result {
	r.t.Helper()
	if len(r.body) != 0 {
		r.t.Errorf("expected empty body, got: %s", r.body)
	}
	return r
}

// Header returns the value of a response header.
func (r *Result) Header(name string) string {
	return r.resp.Header.Get(name)
}

// HasHeader asserts a response header is present and non-empty.
func (r *Result) HasHeader(name string) *Result {
	r.t.Helper()
	if r.resp.Header.Get(name) == "" {
		r.t.Errorf("expected header %q to be present", name)
	}
	return r
}

// HeaderContains asserts a response header contains a substring.
func (r *Result) HeaderContains(name, sub string) *Result {
	r.t.Helper()
	v := r.resp.Header.Get(name)
	if !strings.Contains(v, sub) {
		r.t.Errorf("header %q: got %q, want it to contain %q", name, v, sub)
	}
	return r
}

// ContentType asserts the Content-Type header starts with the given value.
func (r *Result) ContentType(prefix string) *Result {
	r.t.Helper()
	ct := r.resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, prefix) {
		r.t.Errorf("Content-Type: got %q, want prefix %q", ct, prefix)
	}
	return r
}

// BodyContains asserts the raw body contains a substring.
func (r *Result) BodyContains(sub string) *Result {
	r.t.Helper()
	if !strings.Contains(string(r.body), sub) {
		r.t.Errorf("body does not contain %q\nbody: %s", sub, r.body)
	}
	return r
}

// RawBody returns the raw body bytes.
func (r *Result) RawBody() []byte {
	return r.body
}

// ErrorCode asserts the PostgREST error envelope contains the given code.
func (r *Result) ErrorCode(code string) *Result {
	r.t.Helper()
	var env struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(r.body, &env); err != nil {
		r.t.Errorf("parse error envelope: %v\nbody: %s", err, r.body)
		return r
	}
	if env.Code != code {
		r.t.Errorf("error code: got %q, want %q\nbody: %s", env.Code, code, r.body)
	}
	return r
}

// ArrayLen asserts the JSON array body has exactly n elements.
func (r *Result) ArrayLen(n int) *Result {
	r.t.Helper()
	arr := r.JSONArray()
	if len(arr) != n {
		r.t.Errorf("array length: got %d, want %d", len(arr), n)
	}
	return r
}

// RowsHaveField asserts every row in the JSON array contains the given key.
func (r *Result) RowsHaveField(key string) *Result {
	r.t.Helper()
	arr := r.JSONArray()
	for i, row := range arr {
		if _, ok := row[key]; !ok {
			r.t.Errorf("row[%d] missing field %q", i, key)
		}
	}
	return r
}

// RowsAllMatch asserts every row satisfies the predicate.
func (r *Result) RowsAllMatch(desc string, fn func(map[string]any) bool) *Result {
	r.t.Helper()
	arr := r.JSONArray()
	for i, row := range arr {
		if !fn(row) {
			r.t.Errorf("row[%d] does not satisfy %q: %v", i, desc, row)
		}
	}
	return r
}

// P builds url.Values from alternating key-value pairs.
func P(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

// H_ builds a headers map from alternating key-value pairs.
func H_(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// ContentRangeTotal parses the total from "0-4/10". Returns -1 for "*" or bad input.
func ContentRangeTotal(v string) int {
	parts := strings.Split(v, "/")
	if len(parts) != 2 || parts[1] == "*" {
		return -1
	}
	var n int
	fmt.Sscan(parts[1], &n)
	return n
}

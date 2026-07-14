package apidocs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegister(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	get := func(path string) (*http.Response, []byte) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s read: %v", path, err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		return resp, b
	}
	ct := func(resp *http.Response) string { return resp.Header.Get("Content-Type") }

	// /openapi.json — valid JSON with a non-empty paths object.
	resp, body := get("/openapi.json")
	if !strings.HasPrefix(ct(resp), "application/json") {
		t.Errorf("/openapi.json content-type %q", ct(resp))
	}
	if !json.Valid(body) {
		t.Fatal("/openapi.json is not valid JSON")
	}
	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("/openapi.json unmarshal: %v", err)
	}
	if len(spec.Paths) == 0 {
		t.Error("/openapi.json has no paths")
	}

	// /openapi.yaml
	resp, body = get("/openapi.yaml")
	if !strings.HasPrefix(ct(resp), "application/yaml") {
		t.Errorf("/openapi.yaml content-type %q", ct(resp))
	}
	if !strings.Contains(string(body), "openapi:") {
		t.Error("/openapi.yaml missing 'openapi:'")
	}

	// /docs
	resp, body = get("/docs")
	if !strings.HasPrefix(ct(resp), "text/html") {
		t.Errorf("/docs content-type %q", ct(resp))
	}
	if !strings.Contains(string(body), "<redoc") {
		t.Error("/docs missing <redoc> element")
	}

	// /docs/redoc.standalone.js — non-trivial JS bundle.
	resp, body = get("/docs/redoc.standalone.js")
	if !strings.HasPrefix(ct(resp), "application/javascript") {
		t.Errorf("redoc bundle content-type %q", ct(resp))
	}
	if len(body) < 500_000 {
		t.Errorf("redoc bundle suspiciously small: %d bytes", len(body))
	}
	if !strings.Contains(strings.ToLower(string(body)), "redoc") {
		t.Error("redoc bundle missing 'redoc' marker")
	}

	// Non-GET is rejected with 405 + Allow: GET.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/openapi.json", nil)
	post, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /openapi.json: status %d, want 405", post.StatusCode)
	}
	if post.Header.Get("Allow") != "GET" {
		t.Errorf("405 Allow header = %q, want GET", post.Header.Get("Allow"))
	}
}

package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestEveryAPIRouteIsDocumented keeps GET /api/openapi.json in step with the
// routes production actually serves: a new route without a docs entry, or a
// docs entry for a deleted route, fails here.
func TestEveryAPIRouteIsDocumented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterAPI(r)

	param := regexp.MustCompile(`:(\w+)`)
	served := map[string]bool{}
	for _, rt := range r.Routes() {
		p := strings.TrimPrefix(rt.Path, "/api")
		served[strings.ToLower(rt.Method)+" "+param.ReplaceAllString(p, "{$1}")] = true
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	handler.APIDocs(c)
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("openapi.json: %v", err)
	}
	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for m := range ops {
			documented[m+" "+path] = true
		}
	}

	for k := range served {
		if !documented[k] {
			t.Errorf("route %s is served but missing from openapi.json (internal/handler/api_docs.go)", k)
		}
	}
	for k := range documented {
		if !served[k] {
			t.Errorf("openapi.json documents %s, which no route serves", k)
		}
	}
}

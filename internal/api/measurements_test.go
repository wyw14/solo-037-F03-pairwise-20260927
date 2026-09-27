package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/solo-037/internal/service"
)

func TestMeasuredRunHTTPUsesOperatorEvidence(t *testing.T) {
	runtime, err := service.OpenRuntime(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := NewRouter(runtime).Handler()
	created := httptest.NewRecorder()
	server.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/measured-runs", strings.NewReader(`{"name":"tow A","model_name":"hull A","section":"section-a","carriage":"carriage-1"}`)))
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var response struct {
		Data service.MeasuredRun `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id := response.Data.Operation.ID
	if id == "" {
		t.Fatal("missing ID")
	}
	stamp := time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	input := `{"kind":"wave_time","source_id":"probe-a","value":0.24,"recorded_at":"` + stamp + `"}`
	recorded := httptest.NewRecorder()
	server.ServeHTTP(recorded, httptest.NewRequest(http.MethodPost, "/api/measured-runs/"+string(id)+"/evidence", strings.NewReader(input)))
	if recorded.Code != http.StatusOK {
		t.Fatalf("record: %d %s", recorded.Code, recorded.Body)
	}
	analysis := httptest.NewRecorder()
	server.ServeHTTP(analysis, httptest.NewRequest(http.MethodGet, "/api/measured-runs/"+string(id)+"/analysis", nil))
	if analysis.Code != http.StatusOK || !strings.Contains(analysis.Body.String(), `"mean":0.24`) {
		t.Fatalf("analysis: %d %s", analysis.Code, analysis.Body)
	}
	reloaded, err := service.OpenRuntime(t.TempDir())
	if err != nil || len(reloaded.MeasuredRuns()) != 0 {
		t.Fatal("archive separation failed", err)
	}
}

func TestProbeBindingAndPermitEndpoints(t *testing.T) {
	runtime, err := service.OpenRuntime(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := NewRouter(runtime).Handler()
	from := time.Now().Add(-time.Hour).UTC()
	binding := `{"channel":"eth-0","physical":"probe-1","generation":1,"effective_from":"` + from.Format(time.RFC3339Nano) + `"}`
	registered := httptest.NewRecorder()
	server.ServeHTTP(registered, httptest.NewRequest(http.MethodPost, "/api/probe-bindings", strings.NewReader(binding)))
	if registered.Code != http.StatusOK {
		t.Fatalf("register binding: %d %s", registered.Code, registered.Body)
	}
	listed := httptest.NewRecorder()
	server.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/probe-bindings", nil))
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"physical":"probe-1"`) {
		t.Fatalf("list bindings: %d %s", listed.Code, listed.Body)
	}
	created := httptest.NewRecorder()
	server.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/measured-runs", strings.NewReader(`{"name":"tow B","model_name":"hull B","section":"section-a","carriage":"carriage-1"}`)))
	var response struct {
		Data service.MeasuredRun `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id := string(response.Data.Operation.ID)
	stamp := from.Add(time.Minute).Format(time.RFC3339Nano)
	for _, source := range []string{"eth-0", "eth-0"} {
		input := `{"kind":"wave_time","source_id":"` + source + `","value":0.01,"recorded_at":"` + stamp + `"}`
		recorded := httptest.NewRecorder()
		server.ServeHTTP(recorded, httptest.NewRequest(http.MethodPost, "/api/measured-runs/"+id+"/evidence", strings.NewReader(input)))
		if recorded.Code != http.StatusOK {
			t.Fatalf("record: %d %s", recorded.Code, recorded.Body)
		}
	}
	permit := httptest.NewRecorder()
	server.ServeHTTP(permit, httptest.NewRequest(http.MethodGet, "/api/measured-runs/"+id+"/permit", nil))
	if permit.Code != http.StatusOK {
		t.Fatalf("permit: %d %s", permit.Code, permit.Body)
	}
	body := permit.Body.String()
	if !strings.Contains(body, `"independent_sources"`) || !strings.Contains(body, `"supporting"`) || !strings.Contains(body, `"physical":"probe-1"`) {
		t.Fatalf("permit decision should explain its supporting physical sources: %s", body)
	}
}

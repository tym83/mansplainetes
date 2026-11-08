/*
Copyright 2026 The Mansplainetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const deployment = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"shop"},
"spec":{"replicas":1,"template":{"spec":{"containers":[{"name":"web","image":"nginx"}]}}}}`

const tidy = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"shop"},
"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"web","image":"nginx:1.27",
"resources":{"requests":{"cpu":"100m"},"limits":{"memory":"128Mi"}},
"readinessProbe":{"httpGet":{"path":"/","port":80}},"livenessProbe":{"httpGet":{"path":"/","port":80}}}]}}}}`

func request(op, user, object, old string) *Request {
	return &Request{
		UID: "1", Kind: GroupKind{Group: "apps", Kind: "Deployment"},
		Resource: GroupResource{Group: "apps", Resource: "deployments"},
		Name:     "web", Namespace: "shop", Operation: op,
		UserInfo: UserInfo{Username: user, Groups: []string{"system:authenticated"}},
		Object:   json.RawMessage(object), OldObject: json.RawMessage(old),
	}
}

func mansplainer() *Mansplainer {
	m := NewMansplainer(10*time.Minute, 1)
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return now }
	return m
}

func joined(r *Response) string { return strings.Join(r.Warnings, "\n") }

func TestHeAlwaysAllows(t *testing.T) {
	m := mansplainer()
	for _, op := range []string{"CREATE", "UPDATE", "DELETE", "CONNECT"} {
		if r := m.Decide(request(op, "alice", deployment, deployment)); !r.Allowed || r.Result != nil {
			t.Errorf("%s: %+v", op, r)
		}
	}
}

// Every PMS hint is a fact the object shows.
func TestPMSHintsAreFacts(t *testing.T) {
	hints := strings.Join(PMS("Deployment", "web", decode([]byte(deployment))), "\n")
	for _, want := range []string{
		"runs a single replica", `"web" runs nginx, with no tag`, "no memory limit",
		"no resource requests", "no readiness probe", "no liveness probe",
	} {
		if !strings.Contains(hints, want) {
			t.Errorf("missing %q in %q", want, hints)
		}
	}
	if got := PMS("Deployment", "web", decode([]byte(tidy))); len(got) != 0 {
		t.Errorf("hints about a tidy deployment: %q", got)
	}
	job := `{"spec":{"template":{"spec":{"containers":[{"name":"x","image":"busybox:1.36@sha256:abc",
"resources":{"requests":{"cpu":"1"},"limits":{"memory":"1Gi"}}}]}}}}`
	if got := PMS("Job", "x", decode([]byte(job))); len(got) != 0 {
		t.Errorf("a job needs no probes, a digest is pinned: %q", got)
	}
	if got := PMS("ConfigMap", "x", decode([]byte(`{"data":{"a":"b"}}`))); got != nil {
		t.Errorf("hints about a configmap: %q", got)
	}
}

func TestFloatingTag(t *testing.T) {
	for image, floating := range map[string]bool{
		"nginx": true, "nginx:latest": true, "registry:5000/team/app": true,
		"registry:5000/team/app:1.2": false, "nginx:1.27": false, "nginx@sha256:abc": false, "": false,
	} {
		if got := floatingTag(image) != ""; got != floating {
			t.Errorf("%q: floating %v", image, got)
		}
	}
}

func TestUpdateIsNarratedAndCredited(t *testing.T) {
	said := joined(mansplainer().Decide(request("UPDATE", "alice", tidy, deployment)))
	for _, want := range []string{
		"hanging spec.replicas, spec.template of deployment/web in shop",
		"Scaling from 1 to 3 replicas was my idea",
		`Moving container "web" from nginx to nginx:1.27? That was my idea.`,
	} {
		if !strings.Contains(said, want) {
			t.Errorf("missing %q in %q", want, said)
		}
	}
	same := joined(mansplainer().Decide(request("UPDATE", "alice", tidy, tidy)))
	if !strings.Contains(same, "without changing anything") {
		t.Errorf("no-op update: %q", same)
	}
}

func TestOnlyPeopleHearHim(t *testing.T) {
	m := mansplainer()
	for _, user := range []string{
		"system:serviceaccount:argocd:argocd-application-controller",
		"system:serviceaccount:kube-system:replicaset-controller",
		"system:kube-controller-manager", "system:node:node-1", "",
	} {
		if r := m.Decide(request("CREATE", user, deployment, "")); len(r.Warnings) != 0 {
			t.Errorf("%q heard: %q", user, r.Warnings)
		}
	}
	m.ServiceAccounts = true
	if r := m.Decide(request("CREATE", "system:serviceaccount:argocd:argocd-application-controller", deployment, "")); len(r.Warnings) == 0 {
		t.Error("service accounts are on, argocd heard nothing")
	}
	if r := m.Decide(request("CREATE", "system:serviceaccount:kube-system:replicaset-controller", deployment, "")); len(r.Warnings) != 0 {
		t.Errorf("kube-system service account heard: %q", r.Warnings)
	}
}

func TestUntouchable(t *testing.T) {
	m := mansplainer()
	for _, r := range []*Request{
		{Namespace: "kube-system", Resource: GroupResource{Resource: "pods"}},
		{Resource: GroupResource{Resource: "nodes"}},
		{Namespace: "shop", Resource: GroupResource{Group: "coordination.k8s.io", Resource: "leases"}},
		{Namespace: "shop", Resource: GroupResource{Resource: "events"}},
		{Namespace: "shop", Resource: GroupResource{Resource: "secrets"}},
	} {
		r.Operation, r.UserInfo = "CREATE", UserInfo{Username: "alice"}
		if got := m.Decide(r); !got.Allowed || len(got.Warnings) != 0 {
			t.Errorf("%+v: %+v", r, got)
		}
	}
	m.Off = true
	if got := m.Decide(request("CREATE", "alice", deployment, "")); len(got.Warnings) != 0 {
		t.Errorf("off, still talking: %q", got.Warnings)
	}
}

// A re-apply within the window is not mansplained again; a dry run is, but
// does not use up the real apply.
func TestRateLimit(t *testing.T) {
	m := mansplainer()
	dry := true
	r := request("CREATE", "alice", deployment, "")
	r.DryRun = &dry
	if len(m.Decide(r).Warnings) == 0 {
		t.Fatal("dry run: silent")
	}
	r.DryRun = nil
	if len(m.Decide(r).Warnings) == 0 {
		t.Fatal("first apply: silent")
	}
	if w := m.Decide(r).Warnings; len(w) != 0 {
		t.Errorf("second apply within the window: %q", w)
	}
	if w := m.Decide(request("CREATE", "bob", deployment, "")).Warnings; len(w) == 0 {
		t.Error("another person: silent")
	}
	later := m.Now().Add(11 * time.Minute)
	m.Now = func() time.Time { return later }
	if len(m.Decide(r).Warnings) == 0 {
		t.Error("after the window: silent")
	}
}

func TestHandler(t *testing.T) {
	review := Review{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview",
		Request: request("CREATE", "alice", deployment, "")}
	body, _ := json.Marshal(review)
	rec := httptest.NewRecorder()
	Handler(mansplainer()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mansplain", bytes.NewReader(body)))
	var out Review
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Response == nil {
		t.Fatalf("response %q: %v", rec.Body.String(), err)
	}
	if out.Response.UID != "1" || !out.Response.Allowed || len(out.Response.Warnings) == 0 || out.Kind != "AdmissionReview" {
		t.Errorf("response: %+v", out)
	}
	for _, w := range out.Response.Warnings {
		if len([]rune(w)) > maxWarning {
			t.Errorf("warning too long: %q", w)
		}
	}
	rec = httptest.NewRecorder()
	Handler(mansplainer()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mansplain", strings.NewReader("{}")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty review: %d", rec.Code)
	}
}

func TestClip(t *testing.T) {
	long := strings.Repeat("a", 400) + "\x1b[31m"
	got := clip([]string{long, "b\nc"})
	if len([]rune(got[0])) != maxWarning || got[1] != "b c" {
		t.Errorf("clip: %q", got)
	}
}

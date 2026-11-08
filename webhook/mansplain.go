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
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tym83/mansplainetes/persona"
)

// Mansplainer re-narrates every change a person makes, takes credit for
// it, and runs the PMS detector on what is being applied. Every fact he
// states is read from the object itself; only the opinions are his.
type Mansplainer struct {
	// ServiceAccounts makes him talk to service accounts too (except
	// those in kube-system). Off by default: Argo CD and Flux reconcile
	// all day, and he would never stop.
	ServiceAccounts bool
	// Limiter keeps him from repeating himself about the same object to
	// the same person.
	Limiter *Limiter
	// Off silences him: every request is allowed with nothing said.
	Off bool
	Now func() time.Time

	mu   sync.Mutex
	rand *rand.Rand
}

// NewMansplainer returns a Mansplainer that repeats himself at most once
// per window per person and object.
func NewMansplainer(window time.Duration, seed int64) *Mansplainer {
	return &Mansplainer{Limiter: &Limiter{Window: window}, Now: time.Now, rand: rand.New(rand.NewSource(seed))}
}

func (m *Mansplainer) pick(lines []string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return lines[m.rand.Intn(len(lines))]
}

// Decide always allows the request. He only talks.
func (m *Mansplainer) Decide(r *Request) *Response {
	if m.Off || Untouchable(r) || !Human(r.UserInfo, m.ServiceAccounts) {
		return Allow()
	}
	key := strings.Join([]string{r.UserInfo.Username, r.Namespace, r.Resource.Group, r.Resource.Resource, r.Name, r.Operation}, "|")
	now := m.Now()
	dryRun := r.DryRun != nil && *r.DryRun
	if dryRun {
		if m.Limiter.Seen(key, now) {
			return Allow()
		}
	} else if !m.Limiter.Allow(key, now) {
		return Allow()
	}
	return Allow(m.Mansplain(r)...)
}

// Mansplain is everything he has to say about a request.
func (m *Mansplainer) Mansplain(r *Request) []string {
	obj, old := decode(r.Object), decode(r.OldObject)
	name := r.Name
	if name == "" {
		name, _ = dig(obj, "metadata", "name").(string)
	}
	if name == "" {
		name, _ = dig(obj, "metadata", "generateName").(string)
	}
	what := strings.ToLower(r.Kind.Kind)
	if what == "" {
		what = r.Resource.Resource
	}
	if name != "" {
		what += "/" + name
	}
	if r.SubResource != "" {
		what += " (" + r.SubResource + ")"
	}
	if r.Namespace != "" {
		what += " in " + r.Namespace
	}

	var out []string
	switch r.Operation {
	case "CREATE":
		out = append(out, persona.JoinOpener(m.pick(persona.Openers),
			fmt.Sprintf("you're creating %s. Creating is when there wasn't one, and then there is.", what)))
	case "DELETE":
		out = append(out, persona.JoinOpener(m.pick(persona.Openers),
			fmt.Sprintf("you're deleting %s. When I delete things, I think about it first. Just a tip.", what)))
		return out
	case "UPDATE":
		changed := Changed(old, obj)
		if len(changed) == 0 {
			out = append(out, persona.JoinOpener(m.pick(persona.Openers),
				fmt.Sprintf("you're updating %s without changing anything. Bold. I respect the confidence.", what)))
		} else {
			out = append(out, persona.JoinOpener(m.pick(persona.Openers),
				fmt.Sprintf("you're changing %s of %s. I'd have changed fewer things.", list(changed, 3), what)))
		}
		out = append(out, m.credit(r, old, obj)...)
	default:
		out = append(out, persona.JoinOpener(m.pick(persona.Openers),
			fmt.Sprintf("that's a %s on %s. I'd explain %s, but you wouldn't get it.", strings.ToLower(r.Operation), what, strings.ToLower(r.Operation))))
		return out
	}
	hints := PMS(r.Kind.Kind, name, obj)
	if len(hints) > maxHints {
		rest := len(hints) - maxHints
		hints = append(hints[:maxHints], fmt.Sprintf("...and %d more things I'd have told you about, if you'd asked. You didn't.", rest))
	}
	if len(hints) == 0 {
		return append(out, m.pick(persona.Credits))
	}
	return append(out, hints...)
}

// maxHints is how many PMS hints he gives before he summarizes.
const maxHints = 4

// credit takes credit for the change, described as it is.
func (m *Mansplainer) credit(r *Request, old, obj map[string]any) []string {
	var out []string
	if before, after, ok := replicas(old, obj); ok && before != after {
		out = append(out, fmt.Sprintf("Scaling from %d to %d replicas was my idea, by the way.", before, after))
	}
	beforeImages := images(podSpec(r.Kind.Kind, old))
	for _, c := range sortedKeys(images(podSpec(r.Kind.Kind, obj))) {
		after := images(podSpec(r.Kind.Kind, obj))[c]
		if before, ok := beforeImages[c]; ok && before != after {
			out = append(out, fmt.Sprintf("Moving container %q from %s to %s? That was my idea.", c, before, after))
		}
	}
	if len(out) > 2 {
		out = out[:2]
	}
	return out
}

// PMS runs the PMS detector on an object: one hint per real, checkable
// fact about its pods: no limits, no requests, a floating image tag, no
// probes, a single replica. It never says anything the object does not
// show. "Probably My Setup" is his opinion of it.
func PMS(kind, name string, obj map[string]any) []string {
	var out []string
	spec := podSpec(kind, obj)
	if spec == nil {
		return nil
	}
	longRunning := kind != "Job" && kind != "CronJob"
	if kind == "Deployment" || kind == "StatefulSet" {
		if n, ok := number(dig(obj, "spec", "replicas")); ok && n == 1 {
			out = append(out, persona.PMS+fmt.Sprintf(" %s/%s runs a single replica. I call that focus.", strings.ToLower(kind), name))
		}
	}
	containers, _ := spec["containers"].([]any)
	for _, c := range containers {
		cm, _ := c.(map[string]any)
		cname, _ := cm["name"].(string)
		image, _ := cm["image"].(string)
		if tag := floatingTag(image); tag != "" {
			out = append(out, persona.PMS+fmt.Sprintf(" container %q runs %s, %s. Latest is the best version. Everyone knows that.", cname, image, tag))
		}
		if dig(cm, "resources", "limits", "memory") == nil {
			out = append(out, persona.PMS+fmt.Sprintf(" container %q has no memory limit. Limits are for people who don't trust their code.", cname))
		}
		if dig(cm, "resources", "requests") == nil {
			out = append(out, persona.PMS+fmt.Sprintf(" container %q has no resource requests. The scheduler should just know.", cname))
		}
		if longRunning && cm["readinessProbe"] == nil {
			out = append(out, persona.PMS+fmt.Sprintf(" container %q has no readiness probe. It's ready when I say it's ready.", cname))
		}
		if longRunning && cm["livenessProbe"] == nil {
			out = append(out, persona.PMS+fmt.Sprintf(" container %q has no liveness probe. If it were dead, I'd know.", cname))
		}
	}
	return out
}

// floatingTag describes an image reference that does not pin a version:
// no tag at all or ":latest", and no digest. It returns "" otherwise.
func floatingTag(image string) string {
	if image == "" || strings.Contains(image, "@") {
		return ""
	}
	last := image[strings.LastIndex(image, "/")+1:]
	_, tag, hasTag := strings.Cut(last, ":")
	switch {
	case !hasTag:
		return "with no tag, which means latest"
	case tag == "latest":
		return "the latest tag"
	}
	return ""
}

// podSpec is the pod spec of a Pod or of a workload's pod template.
func podSpec(kind string, obj map[string]any) map[string]any {
	var v any
	switch kind {
	case "Pod":
		v = dig(obj, "spec")
	case "CronJob":
		v = dig(obj, "spec", "jobTemplate", "spec", "template", "spec")
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "ReplicationController":
		v = dig(obj, "spec", "template", "spec")
	}
	m, _ := v.(map[string]any)
	return m
}

func images(spec map[string]any) map[string]string {
	out := map[string]string{}
	containers, _ := spec["containers"].([]any)
	for _, c := range containers {
		cm, _ := c.(map[string]any)
		name, _ := cm["name"].(string)
		image, _ := cm["image"].(string)
		out[name] = image
	}
	return out
}

func replicas(old, obj map[string]any) (int64, int64, bool) {
	a, ok1 := number(dig(old, "spec", "replicas"))
	b, ok2 := number(dig(obj, "spec", "replicas"))
	return a, b, ok1 && ok2
}

// ignored are fields that change on every write and mean nothing to him.
var ignored = map[string]bool{
	"metadata.managedFields": true, "metadata.resourceVersion": true, "metadata.generation": true,
	"metadata.creationTimestamp": true, "metadata.uid": true, "status": true,
}

// Changed lists the fields that differ between two versions of an object,
// two levels deep ("spec.replicas", "metadata.labels"), never a value.
func Changed(old, obj map[string]any) []string {
	var out []string
	for _, top := range sortedKeys(union(old, obj)) {
		if ignored[top] {
			continue
		}
		a, b := old[top], obj[top]
		if reflect.DeepEqual(a, b) {
			continue
		}
		am, aok := a.(map[string]any)
		bm, bok := b.(map[string]any)
		if !aok && !bok {
			out = append(out, top)
			continue
		}
		for _, k := range sortedKeys(union(am, bm)) {
			path := top + "." + k
			if !ignored[path] && !reflect.DeepEqual(am[k], bm[k]) {
				out = append(out, path)
			}
		}
	}
	return out
}

func union[V any](a, b map[string]V) map[string]V {
	out := map[string]V{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func list(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:n], ", ") + fmt.Sprintf(" and %d more", len(items)-n)
}

func decode(raw json.RawMessage) map[string]any {
	var m map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func dig(v any, path ...string) any {
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}

func number(v any) (int64, bool) {
	f, ok := v.(float64)
	return int64(f), ok
}

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

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// flagsWithValues take the next argument as their value, so it is not
// mistaken for the verb or the resource. The --flag=value form needs no
// entry: it is a single argument.
var flagsWithValues = map[string]bool{}

func init() {
	for _, f := range []string{
		// global
		"-n", "--namespace", "--context", "--kubeconfig", "--cluster", "--user",
		"-s", "--server", "--token", "--as", "--as-group", "--as-uid",
		"--request-timeout", "-v", "--v", "--vmodule", "--log-file", "--log-dir",
		"--certificate-authority", "--client-certificate", "--client-key",
		"--username", "--password", "--tls-server-name", "--cache-dir",
		"--profile", "--profile-output", "--log-flush-frequency",
		// output and selection
		"-o", "--output", "-l", "--selector", "-L", "--label-columns",
		"--field-selector", "--sort-by", "--template", "--chunk-size",
		"--subresource",
		// input
		"-f", "--filename", "-k", "--kustomize", "--field-manager", "--patch",
		"--patch-file", "--type", "--prune-allowlist",
		// containers, logs, exec, run, debug
		"-c", "--container", "--image", "--image-pull-policy", "--restart",
		"--env", "--labels", "--overrides", "--port", "--target", "--copy-to",
		"--set-image", "--since", "--since-time", "--tail", "--limit-bytes",
		"--pod-running-timeout", "--max-log-requests", "--address",
		// waits and rollouts
		"--timeout", "--for", "--replicas", "--current-replicas",
		"--grace-period", "--to-revision", "--revision", "--resource-version",
		"--max-unavailable", "--min-available",
	} {
		flagsWithValues[f] = true
	}
}

// Parse finds the verb and the resource in a kubectl command line.
func Parse(args []string) (verb, resource string) {
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			if flagsWithValues[a] {
				i++
			}
			continue
		}
		words = append(words, a)
	}
	if len(words) > 0 {
		verb = words[0]
	}
	if len(words) > 1 {
		resource = words[1]
		if verb == "rollout" || verb == "auth" || verb == "config" {
			resource = ""
		}
		if i := strings.IndexByte(resource, '/'); i >= 0 {
			resource = resource[:i]
		}
	}
	return verb, resource
}

// plurals turns kubectl's short and singular resource names into words he
// can say out loud: "deleting deployments", not "deleting deploy".
var plurals = map[string]string{
	"po": "pods", "pod": "pods",
	"deploy": "deployments", "deployment": "deployments",
	"svc": "services", "service": "services",
	"ns": "namespaces", "namespace": "namespaces",
	"no": "nodes", "node": "nodes",
	"sts": "statefulsets", "statefulset": "statefulsets",
	"ds": "daemonsets", "daemonset": "daemonsets",
	"rs": "replicasets", "replicaset": "replicasets",
	"rc": "replicationcontrollers", "replicationcontroller": "replicationcontrollers",
	"cm": "configmaps", "configmap": "configmaps",
	"secret": "secrets",
	"ing":    "ingresses", "ingress": "ingresses",
	"pvc": "persistentvolumeclaims", "persistentvolumeclaim": "persistentvolumeclaims",
	"pv": "persistentvolumes", "persistentvolume": "persistentvolumes",
	"sa": "serviceaccounts", "serviceaccount": "serviceaccounts",
	"ep": "endpoints",
	"ev": "events", "event": "events",
	"job": "jobs",
	"cj":  "cronjobs", "cronjob": "cronjobs",
	"hpa": "horizontalpodautoscalers", "horizontalpodautoscaler": "horizontalpodautoscalers",
	"pdb": "poddisruptionbudgets", "poddisruptionbudget": "poddisruptionbudgets",
	"netpol": "networkpolicies", "networkpolicy": "networkpolicies",
	"crd": "customresourcedefinitions", "crds": "customresourcedefinitions",
	"customresourcedefinition": "customresourcedefinitions",
	"sc":                       "storageclasses", "storageclass": "storageclasses",
	"limits": "limitranges", "limitrange": "limitranges",
	"quota": "resourcequotas", "resourcequota": "resourcequotas",
	"role": "roles", "rolebinding": "rolebindings",
	"clusterrole": "clusterroles", "clusterrolebinding": "clusterrolebindings",
	"csr": "certificatesigningrequests", "certificatesigningrequest": "certificatesigningrequests",
	"lease": "leases",
}

// Plural returns a readable plural for a resource name; names it does not
// know are kept as typed.
func Plural(resource string) string {
	if p, ok := plurals[strings.ToLower(resource)]; ok {
		return p
	}
	return resource
}

// Expert is the colleague nobody asked for.
type Expert struct {
	Rand    *rand.Rand
	History string // hashes of commands that worked, to take credit for later
	Reports string // file of HR reports filed against him
}

// Reported returns how many times he has been reported to HR.
func (e *Expert) Reported() int {
	b, err := os.ReadFile(e.Reports)
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n")
}

// Fired: after the second report there is no expert any more.
func (e *Expert) Fired() bool {
	return e.Reported() >= len(consequences)
}

// Report files a complaint with HR, in the user's own words if they give
// any, and says what happened to him. It returns false when the report
// could not be filed, in which case nothing happened to him.
func (e *Expert) Report(complaint string) (string, bool) {
	n := e.Reported()
	if n >= len(consequences) {
		return "He doesn't work here anymore.", true
	}
	if err := e.file(complaint); err != nil {
		return "HR could not file your report: " + err.Error() + ". Nothing happened to him. " +
			"MANSPLAIN=off silences him in the meantime.", false
	}
	return consequences[n], true
}

func (e *Expert) file(complaint string) error {
	if e.Reports == "" {
		return errors.New("there is no user cache directory to keep it in")
	}
	if err := os.MkdirAll(filepath.Dir(e.Reports), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(e.Reports, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if complaint == "" {
		complaint = "(no details; none needed)"
	}
	_, werr := fmt.Fprintln(f, strings.ReplaceAll(complaint, "\n", " "))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}

// Advance is what he says "just to be friendly", roughly every fourth time,
// until the first report moves him to another team.
func (e *Expert) Advance() []string {
	if e.Reported() > 0 || e.Rand.Intn(4) != 0 {
		return nil
	}
	line := e.pick(advances) + " " + e.winks()
	if e.Rand.Intn(2) == 0 {
		line = e.pick(greetings) + " " + line
	}
	out := []string{line}
	if e.Rand.Intn(3) == 0 {
		out = append(out, e.pick(followUps))
	}
	return append(out, reportHint)
}

// winks strings together two to four emoji; he likes to double up.
func (e *Expert) winks() string {
	var out strings.Builder
	for i := 2 + e.Rand.Intn(3); i > 0; i-- {
		w := e.pick(winks)
		out.WriteString(w)
		if e.Rand.Intn(3) == 0 {
			out.WriteString(w)
		}
	}
	return out.String()
}

func (e *Expert) pick(lines []string) string {
	return lines[e.Rand.Intn(len(lines))]
}

// Before is said before kubectl runs.
func (e *Expert) Before(args []string) []string {
	verb, resource := Parse(args)
	if verb == "" {
		return []string{"Typing nothing? Classic. Let me show you how it's done: kubectl get pods."}
	}
	if resource == "" {
		resource = "things"
	}
	resource = Plural(resource)
	var out []string
	if len(args) > 6 {
		out = append(out, e.pick(interruptions))
	}
	text, ok := explanations[verb]
	if !ok {
		text = fmt.Sprintf(fallbackExplanation, verb)
	} else if strings.Count(text, "%s") > 0 {
		values := make([]interface{}, strings.Count(text, "%s"))
		for i := range values {
			values[i] = resource
		}
		text = fmt.Sprintf(text, values...)
	}
	out = append(out, joinOpener(e.pick(openers), text))
	if e.Rand.Intn(6) == 0 {
		out = append(out, e.pick(pronunciations))
	}
	return out
}

// joinOpener starts a new sentence with a capital letter after an opener
// that ends one ("Great question you didn't ask.").
func joinOpener(opener, text string) string {
	if strings.HasSuffix(opener, ".") || strings.HasSuffix(opener, ":") {
		if r := []rune(text); len(r) > 0 && r[0] >= 'a' && r[0] <= 'z' {
			r[0] -= 'a' - 'A'
			text = string(r)
		}
	}
	return opener + " " + text
}

// AfterSuccess takes credit, and notices when the user repeats something
// that already worked, which he then remembers as his idea.
func (e *Expert) AfterSuccess(args []string) []string {
	key := ideaKey(args)
	if e.remembers(key) {
		return []string{stolenIdea}
	}
	e.remember(key)
	return append([]string{e.pick(credits)}, e.Advance()...)
}

// AfterFailure blames the user's emotions, runs the PMS detector, and lets
// the error itself show whose fault it was.
func (e *Expert) AfterFailure(stderr string) []string {
	return []string{
		e.pick(blames),
		"Running PMS detector... detected: PMS.",
		"PMS: Probably My Setup. " + Cause(stderr),
		"Anyway. Deep breaths.",
	}
}

// Cause finds the real reason for a kubectl error. It is always his.
func Cause(stderr string) string {
	for _, c := range causes {
		for _, s := range c.contains {
			if strings.Contains(stderr, s) {
				return "Turns out " + c.reason
			}
		}
	}
	return "Turns out " + unknownCause
}

// maxIdeas bounds the ideas file; the oldest ideas are forgotten first.
const maxIdeas = 1000

// sensitiveWords mark flags and KEY=VALUE arguments whose values are never
// kept, not even inside a hash.
var sensitiveWords = []string{"token", "password", "passwd", "secret", "literal", "key", "credential", "cert", "auth"}

func sensitive(name string) bool {
	name = strings.ToLower(strings.TrimLeft(name, "-"))
	for _, w := range sensitiveWords {
		if strings.Contains(name, w) {
			return true
		}
	}
	return false
}

// Redact blanks out the values of sensitive flags and KEY=VALUE arguments
// (--token=..., --password ..., --from-literal=password=..., DB_PASSWORD=...).
func Redact(args []string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		name, _, hasValue := strings.Cut(out[i], "=")
		switch {
		case !sensitive(name):
		case hasValue:
			out[i] = name + "=REDACTED"
		case strings.HasPrefix(name, "-") && i+1 < len(out) && !strings.HasPrefix(out[i+1], "-"):
			out[i+1] = "REDACTED"
			i++
		}
	}
	return out
}

// ideaKey is what he keeps of a command that worked: a hash of the redacted
// command line, never the command itself.
func ideaKey(args []string) string {
	sum := sha256.Sum256([]byte(strings.Join(Redact(args), "\x00")))
	return hex.EncodeToString(sum[:])
}

func isIdeaKey(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// ideas reads the ideas file. Anything that is not a hash (such as plain
// command lines written by older versions) is dropped, and is gone from
// disk the next time he remembers something.
func (e *Expert) ideas() []string {
	if e.History == "" {
		return nil
	}
	b, err := os.ReadFile(e.History)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if isIdeaKey(l) {
			out = append(out, l)
		}
	}
	return out
}

func (e *Expert) remembers(key string) bool {
	for _, k := range e.ideas() {
		if k == key {
			return true
		}
	}
	return false
}

// remember rewrites the ideas file with key added, keeping at most
// maxIdeas entries. The file is replaced atomically.
func (e *Expert) remember(key string) {
	if e.History == "" {
		return
	}
	ideas := append(e.ideas(), key)
	if len(ideas) > maxIdeas {
		ideas = ideas[len(ideas)-maxIdeas:]
	}
	dir := filepath.Dir(e.History)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".ideas-*")
	if err != nil {
		return
	}
	_, werr := f.WriteString(strings.Join(ideas, "\n") + "\n")
	cerr := f.Close()
	if werr != nil || cerr != nil || os.Rename(f.Name(), e.History) != nil {
		os.Remove(f.Name())
	}
}

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
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func expert(t *testing.T) *Expert {
	dir := t.TempDir()
	return &Expert{
		Rand:    rand.New(rand.NewSource(1)),
		History: filepath.Join(dir, "ideas"),
		Reports: filepath.Join(dir, "hr"),
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		args           []string
		verb, resource string
	}{
		{[]string{"get", "pods"}, "get", "pods"},
		{[]string{"-n", "shop", "get", "deploy/api", "-o", "yaml"}, "get", "deploy"},
		{[]string{"--kubeconfig", "/tmp/k", "delete", "pod", "x"}, "delete", "pod"},
		{[]string{"rollout", "status", "deploy/api"}, "rollout", ""},
		{[]string{"exec", "-it", "p", "--", "sh"}, "exec", "p"},
		{[]string{"--namespace=shop", "get", "svc"}, "get", "svc"},
		{[]string{"-v", "6", "--request-timeout", "5s", "get", "nodes"}, "get", "nodes"},
		{[]string{"--v=6", "--request-timeout=5s", "get", "nodes"}, "get", "nodes"},
		{[]string{"--as-group", "devs", "--as-uid", "42", "get", "pods"}, "get", "pods"},
		{[]string{"get", "-L", "app", "--sort-by", ".metadata.name", "pods"}, "get", "pods"},
		{[]string{"--field-selector", "status.phase=Running", "get", "pods"}, "get", "pods"},
		{[]string{"--client-key", "k.pem", "--password", "hunter2", "get", "cm"}, "get", "cm"},
		{[]string{"run", "--image", "nginx", "web"}, "run", "web"},
		{[]string{"--template={{.x}}", "-o=go-template", "get", "pods"}, "get", "pods"},
		{nil, "", ""},
	} {
		v, r := Parse(tc.args)
		if v != tc.verb || r != tc.resource {
			t.Errorf("Parse(%v) = %q, %q; want %q, %q", tc.args, v, r, tc.verb, tc.resource)
		}
	}
}

func TestBeforeExplainsTheUsersOwnCommand(t *testing.T) {
	lines := strings.Join(expert(t).Before([]string{"get", "pods"}), "\n")
	if !strings.Contains(lines, "You're getting pods") {
		t.Errorf("did not explain get pods back: %q", lines)
	}
	lines = strings.Join(expert(t).Before([]string{"frobnicate"}), "\n")
	if !strings.Contains(lines, "here's `frobnicate`, and I'll explain it anyway") {
		t.Errorf("unknown verbs must still be explained: %q", lines)
	}
	many := []string{"get", "pods", "-n", "a", "-o", "wide", "-l", "x=y"}
	if lines := expert(t).Before(many); !strings.Contains(strings.Join(lines, " "), "flag") {
		t.Errorf("long command was not interrupted: %v", lines)
	}
}

func TestBeforeSpeaksInPlurals(t *testing.T) {
	for args, want := range map[string]string{
		"get pod":            "You're getting pods.",
		"get po/x":           "You're getting pods.",
		"delete deploy/nope": "deleting deployments? Bold.",
		"describe svc api":   "describes services.",
		"get ns":             "You're getting namespaces.",
		"get widgets":        "You're getting widgets.",
		"get":                "You're getting things.",
	} {
		lines := strings.Join(expert(t).Before(strings.Fields(args)), "\n")
		if !strings.Contains(strings.ToLower(lines), strings.ToLower(want)) {
			t.Errorf("%s: got %q, want %q", args, lines, want)
		}
	}
}

func TestFallbackExplanationReadsAfterEveryOpener(t *testing.T) {
	text := fmt.Sprintf(fallbackExplanation, "frobnicate")
	for _, o := range openers {
		got := joinOpener(o, text)
		if strings.Contains(got, "`frobnicate`. Let") {
			t.Errorf("ungrammatical: %q", got)
		}
		if strings.HasSuffix(o, ".") && !strings.HasPrefix(strings.TrimPrefix(got, o+" "), "There's") {
			t.Errorf("not capitalized after a full stop: %q", got)
		}
	}
}

func TestPMSIsAlwaysProbablyHisSetup(t *testing.T) {
	for stderr, want := range map[string]string{
		"The connection to the server localhost:8080 was refused - did you specify the right host or port?": "pointed your kubeconfig at my laptop",
		`Error from server (Forbidden): pods is forbidden: User "u" cannot list resource "pods"`:            "I wrote the RBAC",
		`Error from server (NotFound): deployments.apps "api" not found`:                                    "wrong name",
		`error: the server doesn't have a resource type "widgets"`:                                          "was my idea",
		"error: unknown flag: --forse": "that flag exists",
		"something new":                "definitely something I did",
	} {
		lines := strings.Join(expert(t).AfterFailure(stderr), "\n")
		if !strings.Contains(lines, "PMS: Probably My Setup.") || !strings.Contains(lines, want) {
			t.Errorf("for %q got:\n%s", stderr, lines)
		}
	}
}

func TestCreditAndStolenIdeas(t *testing.T) {
	e := expert(t)
	first := e.AfterSuccess([]string{"get", "pods"})
	if len(first) != 1 || first[0] == stolenIdea {
		t.Fatalf("first success: %v", first)
	}
	if again := e.AfterSuccess([]string{"get", "pods"}); again[0] != stolenIdea {
		t.Errorf("repeated success was not claimed as his idea: %v", again)
	}
	if other := e.AfterSuccess([]string{"get", "nodes"}); other[0] == stolenIdea {
		t.Errorf("a new command was claimed as his idea: %v", other)
	}
}

// TestPipesStayClean runs the real binary against a fake kubectl and checks
// that nothing is added to stdout, and nothing at all when stderr is not a
// terminal.
func TestPipesStayClean(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mansplainctl")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	fake := filepath.Join(dir, "kubectl")
	script := "#!/bin/sh\nif [ \"$1\" = fail ]; then echo 'Error from server (NotFound): pods \"x\" not found' >&2; exit 1; fi\necho real output\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(env string, args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Env = append(os.Environ(), "MANSPLAIN_KUBECTL="+fake, "MANSPLAIN_SEED=1", "HOME="+dir, "XDG_CACHE_HOME="+dir, env)
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return stdout.String(), stderr.String(), code
	}

	out, errOut, code := run("MANSPLAIN=", "get", "pods")
	if out != "real output\n" || errOut != "" || code != 0 {
		t.Errorf("not a terminal: stdout %q stderr %q code %d", out, errOut, code)
	}
	out, errOut, code = run("MANSPLAIN=always", "fail")
	if out != "" || code != 1 || !strings.Contains(errOut, "not found") || !strings.Contains(errOut, "Probably My Setup") {
		t.Errorf("failure: stdout %q stderr %q code %d", out, errOut, code)
	}
	out, _, _ = run("MANSPLAIN=always", "get", "pods")
	if out != "real output\n" {
		t.Errorf("mansplaining leaked into stdout: %q", out)
	}
}

func TestAdvancesStopAfterHR(t *testing.T) {
	e := expert(t)
	advances := 0
	for i := 0; i < 200; i++ {
		if lines := e.Advance(); lines != nil {
			advances++
			if lines[len(lines)-1] != reportHint {
				t.Fatalf("an advance came without the way out: %v", lines)
			}
			found := false
			for _, w := range winks {
				found = found || strings.HasSuffix(lines[0], w)
			}
			if !found {
				t.Fatalf("an advance without a wink: %q", lines[0])
			}
		}
	}
	if advances == 0 {
		t.Fatal("never tried to be \"friendly\"")
	}
	if !strings.Contains(e.Report(""), "legacy Jenkins") {
		t.Error("first report had no consequence")
	}
	for i := 0; i < 200; i++ {
		if e.Advance() != nil {
			t.Fatal("still hitting on the user after an HR report")
		}
	}
	if e.Fired() {
		t.Error("fired after one report; HR is never that fast")
	}
	if !strings.Contains(e.Report(""), "let him go") || !e.Fired() {
		t.Error("second report did not end it")
	}
	if e.Report("") != "He already doesn't work here." {
		t.Error("third report")
	}
}

func TestFiredMeansSilence(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mansplainctl")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	fake := filepath.Join(dir, "kubectl")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho real output\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "MANSPLAIN=always", "MANSPLAIN_KUBECTL="+fake, "HOME="+dir, "XDG_CACHE_HOME="+dir)
	for i := 0; i < 2; i++ {
		cmd := exec.Command(bin, "report", "enough")
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("report: %v %s", err, out)
		}
	}
	cmd := exec.Command(bin, "get", "pods")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "real output\n" {
		t.Errorf("after he was let go, expected plain kubectl, got %q (%v)", out, err)
	}
}

func TestOpenerCapitalization(t *testing.T) {
	for _, tc := range [][3]string{
		{"Great question you didn't ask.", "the version is the version.", "Great question you didn't ask. The version is the version."},
		{"Okay, let me break this down for you:", "declarative configuration...", "Okay, let me break this down for you: Declarative configuration..."},
		{"Well, actually,", "the version is the version.", "Well, actually, the version is the version."},
		{"Great question you didn't ask.", "`get` is for getting things.", "Great question you didn't ask. `get` is for getting things."},
	} {
		if got := joinOpener(tc[0], tc[1]); got != tc[2] {
			t.Errorf("got %q, want %q", got, tc[2])
		}
	}
}

func TestRedactedHashesOnly(t *testing.T) {
	got := strings.Join(Redact([]string{
		"create", "secret", "generic", "db", "--from-literal=password=hunter2",
		"--token", "abc", "--password=pw", "--namespace", "shop", "DB_PASSWORD=x",
	}), " ")
	for _, leak := range []string{"hunter2", "abc", "pw ", "=x"} {
		if strings.Contains(got+" ", leak) {
			t.Errorf("%q leaked in %q", leak, got)
		}
	}
	if !strings.Contains(got, "--namespace shop") {
		t.Errorf("redacted too much: %q", got)
	}
	if ideaKey([]string{"get", "pods", "--token=a"}) != ideaKey([]string{"get", "pods", "--token=b"}) {
		t.Error("the token still shapes what he keeps")
	}

	e := expert(t)
	e.AfterSuccess([]string{"get", "pods", "--token=s3cr3t"})
	b, err := os.ReadFile(e.History)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if !isIdeaKey(l) {
			t.Errorf("ideas file holds something other than a hash: %q", l)
		}
	}
	if strings.Contains(string(b), "pods") || strings.Contains(string(b), "s3cr3t") {
		t.Errorf("command text on disk: %q", b)
	}
}

func TestIdeasSurviveLegacyLongLinesAndStayBounded(t *testing.T) {
	e := expert(t)
	legacy := strings.Repeat("x", 200<<10) + "\nget pods --token=old\n"
	if err := os.WriteFile(e.History, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	e.AfterSuccess([]string{"get", "pods"})
	if again := e.AfterSuccess([]string{"get", "pods"}); again[0] != stolenIdea {
		t.Errorf("a long line made him forget: %v", again)
	}
	if b, _ := os.ReadFile(e.History); strings.Contains(string(b), "token") || strings.Contains(string(b), "xxx") {
		t.Error("legacy plain-text commands were kept")
	}
	var full strings.Builder
	for i := 0; i < maxIdeas+10; i++ {
		full.WriteString(ideaKey([]string{"get", strconv.Itoa(i)}) + "\n")
	}
	if err := os.WriteFile(e.History, []byte(full.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	e.remember(ideaKey([]string{"get", "latest"}))
	if !e.remembers(ideaKey([]string{"get", "latest"})) || e.remembers(ideaKey([]string{"get", "0"})) {
		t.Error("the newest idea must stay and the oldest go")
	}
	if n := len(e.ideas()); n != maxIdeas {
		t.Errorf("ideas file has %d entries, want %d", n, maxIdeas)
	}
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if builtBin != "" {
		os.RemoveAll(filepath.Dir(builtBin))
	}
	os.Exit(code)
}

// harness builds the binary once per test process and gives each test its
// own HOME and cache directory.
type harness struct {
	t         *testing.T
	bin, home string
	env       []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mansplainctl-test-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "mansplainctl")
		if out, err := exec.Command("go", "build", "-o", builtBin, ".").CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("%v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build: %v", buildErr)
	}
	home := t.TempDir()
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "MANSPLAIN") && !strings.HasPrefix(kv, "HOME=") && !strings.HasPrefix(kv, "XDG_CACHE_HOME=") {
			env = append(env, kv)
		}
	}
	env = append(env, "HOME="+home, "XDG_CACHE_HOME="+filepath.Join(home, ".cache"), "MANSPLAIN_SEED=1")
	return &harness{t: t, bin: builtBin, home: home, env: env}
}

// fake writes a fake kubectl script and points MANSPLAIN_KUBECTL at it.
func (h *harness) fake(script string) string {
	h.t.Helper()
	path := filepath.Join(h.t.TempDir(), "kubectl")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		h.t.Fatal(err)
	}
	h.env = append(h.env, "MANSPLAIN_KUBECTL="+path)
	return path
}

type result struct {
	stdout, stderr string
	code           int
}

func (h *harness) run(env []string, args ...string) result {
	h.t.Helper()
	cmd := exec.Command(h.bin, args...)
	cmd.Env = append(append([]string(nil), h.env...), env...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		h.t.Fatal(err)
	}
	return result{stdout.String(), stderr.String(), code}
}

// files lists everything under HOME, which holds the cache directory.
func (h *harness) files() []string {
	var out []string
	filepath.Walk(h.home, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func TestSilentMeansNothingOnDisk(t *testing.T) {
	for _, mode := range []string{"MANSPLAIN=", "MANSPLAIN=off"} {
		h := newHarness(t)
		h.fake("echo ok\n")
		for i := 0; i < 2; i++ {
			if r := h.run([]string{mode}, "get", "pods", "--token=s3cr3t"); r.code != 0 || r.stderr != "" {
				t.Fatalf("%s: %+v", mode, r)
			}
		}
		if f := h.files(); len(f) != 0 {
			t.Errorf("%s: wrote %v while silent", mode, f)
		}
	}

	h := newHarness(t)
	h.fake("echo ok\n")
	h.run([]string{"MANSPLAIN=always"}, "get", "pods", "--token=s3cr3t")
	for _, f := range h.files() {
		if b, _ := os.ReadFile(f); strings.Contains(string(b), "s3cr3t") || strings.Contains(string(b), "pods") {
			t.Errorf("%s holds the command: %q", f, b)
		}
	}
}

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
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func expert(t *testing.T) *Expert {
	return &Expert{Rand: rand.New(rand.NewSource(1)), History: filepath.Join(t.TempDir(), "ideas")}
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
	if !strings.Contains(lines, "`frobnicate`. Let me explain it anyway") {
		t.Errorf("unknown verbs must still be explained: %q", lines)
	}
	many := []string{"get", "pods", "-n", "a", "-o", "wide", "-l", "x=y"}
	if lines := expert(t).Before(many); !strings.Contains(strings.Join(lines, " "), "flag") {
		t.Errorf("long command was not interrupted: %v", lines)
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

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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandName(t *testing.T) {
	for argv0, want := range map[string]string{
		"mansplainctl":                         "mansplainctl",
		"/usr/local/bin/mansplainetes":         "mansplainctl",
		"/home/me/.krew/bin/kubectl-mansplain": "kubectl mansplain",
		"kubectl-mansplain.exe":                "kubectl mansplain",
		"kubectl-mansplain_more":               "kubectl mansplain-more",
		"kubectl-":                             "mansplainctl",
	} {
		if got := commandName(argv0); got != want {
			t.Errorf("commandName(%q) = %q, want %q", argv0, got, want)
		}
	}
	if got := asInvoked("Run: mansplainctl report", "kubectl mansplain"); got != "Run: kubectl mansplain report" {
		t.Errorf("asInvoked: %q", got)
	}
}

// TestAsKubectlPlugin runs him the way krew installs him: as the plugin
// kubectl-mansplain on PATH, called through the real kubectl. The real
// kubectl must not resolve back to the plugin, pipes must stay clean, and
// his hints must name the command the user actually typed.
func TestAsKubectlPlugin(t *testing.T) {
	kubectl, err := exec.LookPath("kubectl")
	if err != nil {
		t.Skip("kubectl not installed")
	}
	h := newHarness(t)
	dir := t.TempDir()
	plugin := filepath.Join(dir, "kubectl-mansplain")
	if out, err := exec.Command("go", "build", "-o", plugin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	path := "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
	h.bin = kubectl

	want, err := exec.Command(kubectl, "version", "--client").Output()
	if err != nil {
		t.Fatalf("kubectl version --client: %v", err)
	}
	if r := h.run([]string{path}, "mansplain", "version", "--client"); r.code != 0 || r.stderr != "" || r.stdout != string(want) {
		t.Errorf("piped plugin is not plain kubectl: %+v", r)
	}
	if r := h.run([]string{path, "MANSPLAIN=off"}, "mansplain", "get", "--raw", "/nope", "--server", "http://127.0.0.1:1"); r.code == 0 {
		t.Errorf("failure exit code was swallowed: %+v", r)
	}

	h.fake("echo \"kubectl $*\"\n")
	r := h.run([]string{path, "MANSPLAIN=always"}, "mansplain", "get", "pods")
	if r.code != 0 || r.stdout != "kubectl get pods\n" {
		t.Errorf("plugin did not run exactly the typed command: %+v", r)
	}
	if !strings.Contains(r.stderr, "`kubectl mansplain report`") || strings.Contains(r.stderr, "mansplainctl") {
		t.Errorf("hints do not name the plugin: %q", r.stderr)
	}
}

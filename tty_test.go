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
	"testing"
)

// A character device is not a terminal: "2>/dev/null" is a script, and a
// script gets plain kubectl.
func TestOnlyARealTerminalIsATerminal(t *testing.T) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devNull.Close() }()
	if info, err := devNull.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Fatalf("%s is expected to be a character device: %v %v", os.DevNull, info, err)
	}
	if terminal(devNull) {
		t.Errorf("%s counts as a terminal", os.DevNull)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if terminal(w) {
		t.Error("a pipe counts as a terminal")
	}

	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if terminal(f) {
		t.Error("a file counts as a terminal")
	}
}

// With stderr sent to /dev/null he stays silent and kubectl gets that
// stderr directly, not a pipe through him.
func TestStderrToDevNullIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.fake("if [ -p /dev/fd/2 ]; then echo pipe; else echo direct; fi\n")
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devNull.Close() }()
	cmd := exec.Command(h.bin, "get", "pods")
	cmd.Env = append(append([]string(nil), h.env...), "MANSPLAIN=")
	cmd.Stderr = devNull
	out, err := cmd.Output()
	if err != nil || string(out) != "direct\n" {
		t.Errorf("kubectl saw stderr as %q (%v), want \"direct\\n\"", out, err)
	}
	if files := h.files(); len(files) != 0 {
		t.Errorf("he kept something from a script's command: %v", files)
	}
}

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
	"path/filepath"
	"strings"
)

// commandName is how his lines refer to him. kubectl runs a plugin binary
// called kubectl-<name> for "kubectl <name>", so when installed that way
// (for example with krew) he tells you to run "kubectl mansplain report".
func commandName(argv0 string) string {
	base := strings.TrimSuffix(filepath.Base(argv0), ".exe")
	if name, ok := strings.CutPrefix(base, "kubectl-"); ok && name != "" {
		// kubectl maps an underscore in the file name to a dash.
		return "kubectl " + strings.ReplaceAll(name, "_", "-")
	}
	return "mansplainctl"
}

// asInvoked rewrites the commands he suggests to match how he was called.
func asInvoked(line, name string) string {
	if name == "mansplainctl" {
		return line
	}
	return strings.ReplaceAll(line, "mansplainctl", name)
}

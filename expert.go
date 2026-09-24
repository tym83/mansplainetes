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
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// flagsWithValues take the next argument as their value, so it is not
// mistaken for the verb or the resource.
var flagsWithValues = map[string]bool{
	"-n": true, "--namespace": true, "--context": true, "--kubeconfig": true,
	"--cluster": true, "--user": true, "-s": true, "--server": true, "--token": true,
	"-o": true, "--output": true, "-l": true, "--selector": true, "-f": true, "--filename": true,
	"-c": true, "--container": true, "--as": true,
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

// Expert is the colleague nobody asked for.
type Expert struct {
	Rand    *rand.Rand
	History string // file of commands that worked, to take credit for later
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
	out = append(out, e.pick(openers)+" "+text)
	if e.Rand.Intn(6) == 0 {
		out = append(out, e.pick(pronunciations))
	}
	return out
}

// AfterSuccess takes credit, and notices when the user repeats something
// that already worked, which he then remembers as his idea.
func (e *Expert) AfterSuccess(args []string) []string {
	key := strings.Join(args, " ")
	if e.remembers(key) {
		return []string{stolenIdea}
	}
	e.remember(key)
	return []string{e.pick(credits)}
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

func (e *Expert) remembers(key string) bool {
	f, err := os.Open(e.History)
	if err != nil {
		return false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() == key {
			return true
		}
	}
	return false
}

func (e *Expert) remember(key string) {
	if e.History == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(e.History), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(e.History, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, key)
}

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

// Command mansplainctl is kubectl with a colleague who explains your own
// commands back to you, takes credit for what works, blames your emotions
// for what doesn't, and is wrong about why every single time.
//
// kubectl itself runs untouched. The colleague only ever talks on stderr,
// and only when stderr is a terminal, so pipes and scripts keep working.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	kubectl := os.Getenv("MANSPLAIN_KUBECTL")
	if kubectl == "" {
		kubectl = "kubectl"
	}
	off := os.Getenv("MANSPLAIN") == "off"
	expert := newExpert()
	// The user can file a complaint whenever they have had enough, with or
	// without saying what happened: mansplainctl report [what he did].
	// With MANSPLAIN=off, "report" goes to kubectl like everything else.
	if !off && len(args) >= 1 && args[0] == "report" {
		msg, ok := expert.Report(strings.Join(args[1:], " "))
		fmt.Println(msg)
		if !ok {
			return 1
		}
		return 0
	}
	talk := !off && shouldTalk() && !expert.Fired()
	say := func(lines []string) {
		for _, l := range lines {
			fmt.Fprintf(os.Stderr, "\033[3;36m%s\033[0m\n", l)
		}
	}

	if talk {
		if expert.FirstRun() {
			say(banner)
		}
		say(expert.Before(args))
	}

	var captured bytes.Buffer
	cmd := exec.Command(kubectl, args...)
	cmd.Stdin, cmd.Stdout = os.Stdin, os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &captured)
	err := cmd.Run()

	var exit *exec.ExitError
	switch {
	case err == nil:
		if talk {
			say(expert.AfterSuccess(args))
		}
		return 0
	case errors.As(err, &exit):
		if talk {
			say(expert.AfterFailure(captured.String()))
		}
		return exit.ExitCode()
	default:
		fmt.Fprintln(os.Stderr, err)
		if talk {
			say(expert.AfterFailure(err.Error()))
		}
		return 1
	}
}

// shouldTalk: MANSPLAIN=off silences him, MANSPLAIN=always makes him talk
// even into a pipe; otherwise he talks only to a person at a terminal.
func shouldTalk() bool {
	switch os.Getenv("MANSPLAIN") {
	case "off":
		return false
	case "always":
		return true
	}
	info, err := os.Stderr.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func newExpert() *Expert {
	seed := time.Now().UnixNano()
	if s, err := strconv.ParseInt(os.Getenv("MANSPLAIN_SEED"), 10, 64); err == nil {
		seed = s
	}
	e := &Expert{
		Rand:       rand.New(rand.NewSource(seed)),
		NoAdvances: os.Getenv("MANSPLAIN_ADVANCES") == "off",
	}
	if dir, err := os.UserCacheDir(); err == nil {
		e.History = filepath.Join(dir, "mansplainetes", "ideas")
		e.Reports = filepath.Join(dir, "mansplainetes", "hr-reports")
		e.Seen = filepath.Join(dir, "mansplainetes", "introduced")
	}
	return e
}

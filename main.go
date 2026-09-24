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
	talk := shouldTalk()
	expert := newExpert()
	say := func(lines []string) {
		if !talk {
			return
		}
		for _, l := range lines {
			fmt.Fprintf(os.Stderr, "\033[3;36m%s\033[0m\n", l)
		}
	}

	say(expert.Before(args))

	var captured bytes.Buffer
	cmd := exec.Command(kubectl, args...)
	cmd.Stdin, cmd.Stdout = os.Stdin, os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &captured)
	err := cmd.Run()

	var exit *exec.ExitError
	switch {
	case err == nil:
		say(expert.AfterSuccess(args))
		return 0
	case errors.As(err, &exit):
		say(expert.AfterFailure(captured.String()))
		return exit.ExitCode()
	default:
		fmt.Fprintln(os.Stderr, err)
		say(expert.AfterFailure(err.Error()))
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
	history := ""
	if dir, err := os.UserCacheDir(); err == nil {
		history = filepath.Join(dir, "mansplainetes", "ideas")
	}
	return &Expert{Rand: rand.New(rand.NewSource(seed)), History: history}
}

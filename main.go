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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// depthEnv counts how deeply mansplainctl is nested inside itself, e.g.
	// through a kubectl plugin that runs kubectl, which is him again.
	depthEnv = "MANSPLAIN_DEPTH"
	// maxDepth stops a wrapper that ends up calling itself in a loop.
	maxDepth = 8
	// tailSize is how much of kubectl's stderr he keeps to find the cause
	// of a failure; the rest only goes to the terminal.
	tailSize = 4096
	// waitDelay bounds how long he waits for kubectl's stderr to close
	// after kubectl exits, in case a background process inherited it.
	waitDelay = time.Second
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	depth, _ := strconv.Atoi(os.Getenv(depthEnv))
	if depth >= maxDepth {
		fmt.Fprintf(os.Stderr, "mansplainctl: called itself %d times in a row; set MANSPLAIN_KUBECTL to the real kubectl\n", depth)
		return 1
	}
	kubectl, err := resolveKubectl()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
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
	// Nested calls come from scripts and plugins, not from the person.
	talk := !off && depth == 0 && shouldTalk() && !expert.Fired()
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

	cmd := exec.Command(kubectl, args...)
	cmd.Env = append(os.Environ(), depthEnv+"="+strconv.Itoa(depth+1))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	var tail tailBuffer
	if talk {
		cmd.Stderr = io.MultiWriter(os.Stderr, &tail)
	}
	cmd.WaitDelay = waitDelay
	err = runForwardingSignals(cmd)

	var exit *exec.ExitError
	switch {
	case err == nil || errors.Is(err, exec.ErrWaitDelay):
		if talk {
			say(expert.AfterSuccess(args))
		}
		return 0
	case errors.As(err, &exit):
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		if talk {
			say(expert.AfterFailure(tail.String()))
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

// resolveKubectl finds the kubectl to run and refuses to run itself, which
// happens when mansplainctl is installed as "kubectl" earlier in PATH.
func resolveKubectl() (string, error) {
	name := os.Getenv("MANSPLAIN_KUBECTL")
	if name == "" {
		name = "kubectl"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		// Let running it report the problem, as kubectl would be missing.
		return name, nil
	}
	if self, err := os.Executable(); err == nil && sameFile(self, path) {
		return "", fmt.Errorf("mansplainctl: %q resolves to mansplainctl itself (%s); "+
			"put the real kubectl earlier in PATH or point MANSPLAIN_KUBECTL at it", name, path)
	}
	return path, nil
}

func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	return err == nil && os.SameFile(ai, bi)
}

// runForwardingSignals runs cmd and passes SIGINT, SIGTERM and SIGHUP on to
// it instead of dying first, so kubectl gets to clean up and its exit
// status is what the caller sees.
func runForwardingSignals(cmd *exec.Cmd) error {
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	return err
}

// tailBuffer keeps only the last tailSize bytes written to it.
type tailBuffer struct{ buf []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailSize {
		n := copy(t.buf, t.buf[len(t.buf)-tailSize:])
		t.buf = t.buf[:n]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.buf) }

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

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
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNarratesTheBootFromKinitsFacts(t *testing.T) {
	n := &Narrator{Self: "mansplainetes-boot", Rand: rand.New(rand.NewSource(1))}
	t0 := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	step := func(at time.Duration, services ...Service) string {
		return strings.Join(n.Step(services, nil, t0.Add(at), 40*time.Second+at), "\n")
	}

	if got := step(0, Service{Name: "mansplainetes-boot", State: "Running", PID: 1}); got != "" {
		t.Errorf("narrated himself: %q", got)
	}
	got := step(time.Second,
		Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Waiting"})
	if !strings.Contains(got, "he service containerd is running now, pid 101.") || !strings.Contains(got, "kubelet is waiting for what it depends on") {
		t.Errorf("start: %q", got)
	}
	if got := step(2*time.Second, Service{Name: "containerd", State: "Running", PID: 101}, Service{Name: "kubelet", State: "Waiting"}); got != "" {
		t.Errorf("nothing changed, still talking: %q", got)
	}
	got = step(3*time.Second,
		Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Restarting", Restarts: 1, LastExit: "exit status 1"})
	if !strings.Contains(got, "kubelet exited (exit status 1), restart #1.") || !strings.Contains(got, "PMS: Probably My Setup.") {
		t.Errorf("restart: %q", got)
	}
	got = step(5*time.Second,
		Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Running", PID: 202, Restarts: 1})
	if !strings.Contains(got, "kubelet is back up after 1 restarts") || strings.Contains(got, "All 2") {
		t.Errorf("back up: %q", got)
	}
	got = step(5*time.Second+settle,
		Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Running", PID: 202, Restarts: 1})
	if !strings.Contains(got, "All 2 services are running, 1m15s after power-on.") {
		t.Errorf("settled: %q", got)
	}
	if got := step(10*time.Minute, Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Running", PID: 202, Restarts: 1}); got != "" {
		t.Errorf("settled twice: %q", got)
	}
	// A quick restart between two looks still counts.
	got = step(11*time.Minute, Service{Name: "containerd", State: "Running", PID: 101},
		Service{Name: "kubelet", State: "Running", PID: 303, Restarts: 2, LastExit: "signal: killed"})
	if !strings.Contains(got, "kubelet exited (signal: killed), restart #2.") {
		t.Errorf("quick restart: %q", got)
	}
}

func TestSocketDownIsSaidOnce(t *testing.T) {
	n := &Narrator{Rand: rand.New(rand.NewSource(1))}
	now := time.Now()
	if got := n.Step(nil, errors.New("connection refused"), now, 0); len(got) != 1 || !strings.Contains(got[0], "connection refused") {
		t.Errorf("first: %q", got)
	}
	if got := n.Step(nil, errors.New("connection refused"), now, 0); got != nil {
		t.Errorf("again: %q", got)
	}
}

func TestInstalling(t *testing.T) {
	if !installing("console=ttyS0 kuberoot.mode=install quiet") || installing("console=ttyS0 kuberoot.mode=installer") {
		t.Error("installing")
	}
}

// The client only ever reads: GET /v1/services.
func TestKinitClient(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "kinit.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Skip("no unix sockets:", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/services" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Service{{Name: "kubelet", State: "Running", PID: 7}})
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	got, err := kinitClient(socket)(context.Background())
	if err != nil || len(got) != 1 || got[0].Name != "kubelet" || got[0].PID != 7 {
		t.Errorf("%+v %v", got, err)
	}
}

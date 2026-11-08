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

// Command mansplainetes-boot mansplains a kuberoot node's boot on its
// console. kinit starts it as the first service of the node; it reads the
// status of the node's services from kinit's control socket (read-only:
// GET /v1/services) and narrates every change in his voice: a service
// starting, waiting, exiting and restarting, with the exit kinit
// recorded. The facts are kinit's; the credit-taking is his.
//
// It never exits on its own: kinit restarts a service that exits, and a
// node whose services restart fails its upgrade probation.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tym83/mansplainetes/persona"
)

func main() {
	socket := flag.String("socket", "/run/kuberoot/kinit.sock", "kinit's control socket")
	console := flag.String("console", "/dev/console", "where he talks besides his log")
	every := flag.Duration("every", 2*time.Second, "how often he checks on the services")
	self := flag.String("name", "mansplainetes-boot", "his own service name, which he does not narrate")
	flag.Parse()
	log.SetFlags(0)

	if os.Getenv("MANSPLAIN") == "off" {
		wait(nil)
		return
	}
	// Booted from media, the console belongs to kuberoot's installer screen.
	if cmdline, err := os.ReadFile("/proc/cmdline"); err == nil && installing(string(cmdline)) {
		*console = ""
	}
	out := &speaker{console: *console}
	out.say(banner)

	n := &Narrator{Self: *self, Rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
	client := kinitClient(*socket)
	tick := time.NewTicker(*every)
	defer tick.Stop()
	wait(func() {
		ctx, cancel := context.WithTimeout(context.Background(), *every)
		defer cancel()
		services, err := client(ctx)
		out.say(n.Step(services, err, time.Now(), uptime()))
	}, tick.C)
}

// banner says what this is, once per boot.
var banner = []string{
	"mansplainetes: a satire of mansplaining. He is the joke, not you.",
	"He will now narrate this node's boot from kinit's own service list, and take credit for it.",
}

// wait runs step on every tick until SIGTERM or SIGINT.
func wait(step func(), ticks ...<-chan time.Time) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	var tick <-chan time.Time
	if len(ticks) > 0 {
		tick = ticks[0]
	}
	if step != nil {
		step()
	}
	for {
		select {
		case <-stop:
			return
		case <-tick:
			step()
		}
	}
}

// Service is the part of kinit's ServiceStatus he reads.
type Service struct {
	Name     string `json:"name"`
	State    string `json:"state"` // Running, Restarting or Waiting
	PID      int    `json:"pid,omitempty"`
	Restarts int    `json:"restarts"`
	LastExit string `json:"lastExit,omitempty"`
}

func kinitClient(socket string) func(context.Context) ([]Service, error) {
	c := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	return func(ctx context.Context) ([]Service, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://kinit/v1/services", nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("kinit: %s", resp.Status)
		}
		var out []Service
		return out, json.NewDecoder(resp.Body).Decode(&out)
	}
}

// settle is how long every service must stay up before he calls the boot
// done, and takes credit for it.
const settle = 30 * time.Second

// Narrator turns kinit's service list into his commentary, one change at
// a time.
type Narrator struct {
	Self string
	Rand *rand.Rand

	seen        map[string]Service
	lastChange  time.Time
	settled     bool
	unreachable bool
}

func (n *Narrator) pick(lines []string) string { return lines[n.Rand.Intn(len(lines))] }

// Step compares what kinit says now with what it said before and returns
// what he has to say about it.
func (n *Narrator) Step(services []Service, err error, now time.Time, up time.Duration) []string {
	if err != nil {
		if n.unreachable {
			return nil
		}
		n.unreachable = true
		return []string{"kinit isn't answering on its control socket. I'd have started it earlier. (" + err.Error() + ")"}
	}
	n.unreachable = false
	if n.seen == nil {
		n.seen = map[string]Service{}
		n.lastChange = now
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	var out []string
	running, total := 0, 0
	for _, s := range services {
		if s.Name == n.Self {
			continue
		}
		total++
		if s.State == "Running" {
			running++
		}
		prev, known := n.seen[s.Name]
		n.seen[s.Name] = s
		if known && prev.State == s.State && prev.Restarts == s.Restarts {
			continue
		}
		n.lastChange = now
		switch {
		case s.Restarts > prev.Restarts || (s.State == "Restarting" && (!known || prev.State != "Restarting")):
			n.settled = false
			exit := s.LastExit
			if exit == "" {
				exit = "no reason given"
			}
			out = append(out,
				fmt.Sprintf("%s exited (%s), restart #%d. %s", s.Name, exit, s.Restarts, n.pick(blames)),
				persona.PMS+" "+n.pick(causes))
		case s.State == "Waiting":
			out = append(out, fmt.Sprintf("%s is waiting for what it depends on. Patience. I have plenty.", s.Name))
		case s.State == "Running" && s.Restarts == 0:
			out = append(out, persona.JoinOpener(n.pick(persona.Openers),
				fmt.Sprintf("the service %s is running now, pid %d. %s", s.Name, s.PID, n.pick(claims))))
		case s.State == "Running":
			out = append(out, fmt.Sprintf("%s is back up after %d restarts. See? I fixed it.", s.Name, s.Restarts))
		}
	}
	if !n.settled && total > 0 && running == total && now.Sub(n.lastChange) >= settle {
		n.settled = true
		out = append(out, fmt.Sprintf("All %d services are running, %s after power-on. %s",
			total, up.Truncate(time.Second), n.pick(persona.Credits)))
	}
	return out
}

// claims take credit for a service that started.
var claims = []string{
	"That was my idea.",
	"I told it to.",
	"You're welcome.",
	"I'd have started it faster, but fine.",
}

// blames explain a restart by everyone's state of mind but his.
var blames = []string{
	"Everyone calm down. Running PMS detector... detected: PMS.",
	"It's just being emotional. Running PMS detector... detected: PMS.",
	"Deep breaths, node. Running PMS detector... detected: PMS.",
}

// causes are his confessions; the exit above is kinit's fact.
var causes = []string{
	"I wrote its flags from memory.",
	"I tuned its config last night. Confidently.",
	"I told it the wrong address. Confidently.",
	"I \"cleaned up\" a file it needed.",
}

// speaker writes to his log (stdout, which kinit keeps) and to the console.
type speaker struct{ console string }

func (s *speaker) say(lines []string) {
	if len(lines) == 0 {
		return
	}
	var plain, styled bytes.Buffer
	for _, l := range lines {
		plain.WriteString(l + "\n")
		styled.WriteString("\033[3;36m" + l + "\033[0m\r\n")
	}
	_, _ = os.Stdout.Write(plain.Bytes())
	if s.console == "" {
		return
	}
	c, err := os.OpenFile(s.console, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("console %s: %v", s.console, err)
		return
	}
	defer c.Close()
	_, _ = c.Write(styled.Bytes())
}

// installing says whether the kernel command line boots kuberoot's installer.
func installing(cmdline string) bool {
	for _, arg := range strings.Fields(cmdline) {
		if arg == "kuberoot.mode=install" {
			return true
		}
	}
	return false
}

// uptime is how long the machine has been up, from /proc/uptime.
func uptime() time.Duration {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(strings.Fields(string(b) + " 0")[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

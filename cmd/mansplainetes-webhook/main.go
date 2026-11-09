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

// Command mansplainetes-webhook is Mansplainetes for the whole cluster: a
// validating admission webhook that allows every request and mansplains
// it back to the person who sent it, as admission warnings.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/tym83/mansplainetes/webhook"
)

func main() {
	addr := flag.String("addr", ":8443", "address to serve HTTPS on")
	cert := flag.String("tls-cert", "/tls/tls.crt", "TLS certificate")
	key := flag.String("tls-key", "/tls/tls.key", "TLS private key")
	window := flag.Duration("quiet-for", 10*time.Minute, "how long he keeps quiet about the same object to the same person")
	sas := flag.Bool("service-accounts", false, "talk to service accounts too (never those in kube-system)")
	drain := flag.Duration("drain", 5*time.Second, "after SIGTERM, keep serving this long, until the API server stops sending requests here")
	flag.Parse()

	m := webhook.NewMansplainer(*window, seed())
	m.ServiceAccounts = *sas
	m.Off = os.Getenv("MANSPLAIN") == "off"

	mux := http.NewServeMux()
	mux.Handle("/mansplain", webhook.Handler(m))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })

	certs := &reloader{cert: *cert, key: *key}
	if _, err := certs.get(nil); err != nil {
		log.Fatalf("mansplainetes-webhook: %v", err)
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: certs.get},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		// A terminating pod leaves the Service's endpoints only after the
		// fact, and every kube-proxy takes its time to notice. Closing the
		// listener at once would refuse the requests still routed here, and
		// with failurePolicy Ignore they pass unseen.
		log.Printf("mansplainetes-webhook: draining for %s", *drain)
		time.Sleep(*drain)
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("mansplainetes-webhook: listening on %s (off=%v)", *addr, m.Off)
	if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("mansplainetes-webhook: %v", err)
	}
}

func seed() int64 {
	if s, err := strconv.ParseInt(os.Getenv("MANSPLAIN_SEED"), 10, 64); err == nil {
		return s
	}
	return time.Now().UnixNano()
}

// reloader serves the certificate from disk and picks up a renewed one
// (cert-manager, a rotated Secret) within a minute, without a restart.
type reloader struct {
	cert, key string

	mu     sync.Mutex
	cached *tls.Certificate
	loaded time.Time
}

func (r *reloader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != nil && time.Since(r.loaded) < time.Minute {
		return r.cached, nil
	}
	c, err := tls.LoadX509KeyPair(r.cert, r.key)
	if err != nil {
		if r.cached != nil {
			return r.cached, nil
		}
		return nil, err
	}
	r.cached, r.loaded = &c, time.Now()
	return r.cached, nil
}

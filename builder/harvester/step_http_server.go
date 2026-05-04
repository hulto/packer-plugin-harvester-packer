// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packernet "github.com/hashicorp/packer-plugin-sdk/net"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepHTTPServer serves http_directory content used by autoinstall boot args.
// Unlike the SDK generic step, this version logs every request so we can
// verify whether the guest actually reaches the HTTP endpoint.
type StepHTTPServer struct {
	HTTPDir     string
	HTTPPortMin int
	HTTPPortMax int
	HTTPIP      string

	listener *packernet.Listener
	hits     atomic.Int64
}

func (s *StepHTTPServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)

	if s.HTTPDir == "" {
		state.Put("http_port", 0)
		return multistep.ActionContinue
	}

	if _, err := os.Stat(s.HTTPDir); err != nil {
		err := fmt.Errorf("error finding %q: %w", s.HTTPDir, err)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	listener, err := packernet.ListenRangeConfig{
		Min: s.HTTPPortMin,
		Max: s.HTTPPortMax,
	}.Listen(ctx)
	if err != nil {
		err := fmt.Errorf("error finding HTTP port: %w", err)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	s.listener = listener

	ui.Say(fmt.Sprintf("Starting HTTP server on port %d (dir: %s)", s.listener.Port, s.HTTPDir))

	handler := http.FileServer(http.Dir(s.HTTPDir))
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.hits.Add(1)
			ui.Say(fmt.Sprintf("HTTP request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr))
			handler.ServeHTTP(w, r)
		}),
	}

	go server.Serve(s.listener)

	state.Put("http_port", s.listener.Port)
	if s.HTTPIP != "" {
		state.Put("http_ip", s.HTTPIP)
		ui.Say(fmt.Sprintf("Advertising HTTPIP as %s", s.HTTPIP))
	}
	return multistep.ActionContinue
}

func (s *StepHTTPServer) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			ui.Error(fmt.Sprintf("Failed closing HTTP server on port %d: %s", s.listener.Port, err))
		}
	}
	if s.HTTPDir != "" {
		ui.Say(fmt.Sprintf("HTTP server handled %d request(s)", s.hits.Load()))
	}
}

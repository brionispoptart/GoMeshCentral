//go:build !linux && !windows

package main

import "gomeshcentral/internal/hub"

type unsupportedTerminalManager struct {
	write func(hub.AgentEnvelope) error
}

func newTerminalManager(write func(hub.AgentEnvelope) error) terminalManager {
	return &unsupportedTerminalManager{write: write}
}

func (m *unsupportedTerminalManager) Handle(msg hub.AgentEnvelope) {
	if msg.Type != "terminal_open" {
		return
	}
	_ = m.write(hub.AgentEnvelope{Type: "terminal_error", SessionID: msg.SessionID, Error: "terminal sessions are not supported on this OS in the current build"})
	_ = m.write(hub.AgentEnvelope{Type: "terminal_exit", SessionID: msg.SessionID, ExitCode: 1})
}

func (m *unsupportedTerminalManager) CloseAll() {}

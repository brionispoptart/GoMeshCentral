//go:build linux

package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"

	"gomeshcentral/internal/hub"
)

type linuxTerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*linuxTerminalSession
	write    func(hub.AgentEnvelope) error
}

type linuxTerminalSession struct {
	cmd       *exec.Cmd
	ptyFile   *os.File
	closeOnce sync.Once
}

func (s *linuxTerminalSession) close() {
	s.closeOnce.Do(func() {
		if s.ptyFile != nil {
			_ = s.ptyFile.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
			_ = s.cmd.Process.Kill()
		}
	})
}

func newTerminalManager(write func(hub.AgentEnvelope) error) terminalManager {
	return &linuxTerminalManager{
		sessions: map[string]*linuxTerminalSession{},
		write:    write,
	}
}

func (m *linuxTerminalManager) Handle(msg hub.AgentEnvelope) {
	switch msg.Type {
	case "terminal_open":
		m.open(msg.SessionID, uint16(msg.Cols), uint16(msg.Rows))
	case "terminal_data":
		m.writeInput(msg.SessionID, msg.Data)
	case "terminal_resize":
		m.resize(msg.SessionID, uint16(msg.Cols), uint16(msg.Rows))
	case "terminal_close":
		m.closeSession(msg.SessionID)
	}
}

func (m *linuxTerminalManager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.closeSession(id)
	}
}

// open spawns an interactive login shell attached to a real PTY (not plain
// stdio pipes), matching the ConPTY-backed Windows terminal: correct TERM
// semantics, window size, job control, and full-screen apps (vim/top/less)
// all work, including when the agent runs headless under systemd (no session
// TTY of its own to inherit).
func (m *linuxTerminalManager) open(sessionID string, cols, rows uint16) {
	if sessionID == "" {
		return
	}
	m.closeSession(sessionID)

	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 40
	}

	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-i")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		m.sendError(sessionID, "failed to start shell: "+err.Error())
		return
	}

	session := &linuxTerminalSession{cmd: cmd, ptyFile: ptyFile}
	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	go m.streamOutput(sessionID, ptyFile)
	go m.waitForExit(sessionID, session)
}

func (m *linuxTerminalManager) writeInput(sessionID, data string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok || session == nil || session.ptyFile == nil {
		return
	}
	if _, err := io.WriteString(session.ptyFile, data); err != nil {
		log.Printf("terminal input write failed (%s): %v", sessionID, err)
	}
}

func (m *linuxTerminalManager) resize(sessionID string, cols, rows uint16) {
	if cols == 0 || rows == 0 {
		return
	}
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok || session == nil || session.ptyFile == nil {
		return
	}
	if err := pty.Setsize(session.ptyFile, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		log.Printf("terminal resize failed (%s): %v", sessionID, err)
	}
}

func (m *linuxTerminalManager) streamOutput(sessionID string, reader io.Reader) {
	buf := make([]byte, 2048)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_ = m.write(hub.AgentEnvelope{Type: "terminal_data", SessionID: sessionID, Data: string(buf[:n])})
		}
		if err != nil {
			// The PTY master read returns EIO once the child exits and closes
			// its end; that is expected shutdown, not a real error.
			if err != io.EOF && !isLinuxPtyClosedError(err) {
				log.Printf("terminal stream read failed (%s): %v", sessionID, err)
			}
			return
		}
	}
}

func isLinuxPtyClosedError(err error) bool {
	return err == syscall.EIO || err == os.ErrClosed
}

func (m *linuxTerminalManager) waitForExit(sessionID string, session *linuxTerminalSession) {
	err := session.cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	session.close()

	_ = m.write(hub.AgentEnvelope{Type: "terminal_exit", SessionID: sessionID, ExitCode: exitCode})
}

func (m *linuxTerminalManager) closeSession(sessionID string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if !ok || session == nil {
		return
	}
	session.close()
}

func (m *linuxTerminalManager) sendError(sessionID, message string) {
	_ = m.write(hub.AgentEnvelope{Type: "terminal_error", SessionID: sessionID, Error: message})
	_ = m.write(hub.AgentEnvelope{Type: "terminal_exit", SessionID: sessionID, ExitCode: 1})
}

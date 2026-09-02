//go:build windows

package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"gomeshcentral/internal/hub"

	"golang.org/x/sys/windows"
)

// limitedTerminalBanner is echoed at the top of a session when the agent is not
// elevated, so the operator knows administrator-level commands will fail.
const limitedTerminalBanner = "\x1b[33m[GoMeshCentral] Unattended access is OFF \u2014 this session runs with standard user privileges.\r\n" +
	"Commands that require administrator rights will fail. Enable \"Unattended Access\" from the agent tray icon for full elevated control.\x1b[0m\r\n\r\n"

type windowsTerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*windowsTerminalSession
	write    func(hub.AgentEnvelope) error
}

type windowsTerminalSession struct {
	pty           *windowsPseudoTerminal
	processHandle windows.Handle
	closeOnce     sync.Once
	processOnce   sync.Once
}

func (s *windowsTerminalSession) close() {
	s.closeOnce.Do(func() {
		if s.pty != nil {
			_ = s.pty.Close()
		}
		if s.processHandle != 0 && s.processHandle != windows.InvalidHandle {
			_ = windows.TerminateProcess(s.processHandle, 1)
		}
	})
}

func newTerminalManager(write func(hub.AgentEnvelope) error) terminalManager {
	return &windowsTerminalManager{
		sessions: map[string]*windowsTerminalSession{},
		write:    write,
	}
}

func (m *windowsTerminalManager) Handle(msg hub.AgentEnvelope) {
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

func (m *windowsTerminalManager) CloseAll() {
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

func (m *windowsTerminalManager) open(sessionID string, cols, rows uint16) {
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

	cmd := exec.Command("cmd.exe", "/Q")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Dir = `C:\`

	pty, processHandle, err := startWindowsTerminalProcess(cmd, cols, rows)
	if err != nil {
		log.Printf("terminal open failed session=%s: %v", sessionID, err)
		m.sendError(sessionID, err.Error())
		return
	}

	m.mu.Lock()
	m.sessions[sessionID] = &windowsTerminalSession{pty: pty, processHandle: processHandle}
	m.mu.Unlock()

	// Surface the current privilege level so the operator understands why an
	// elevated command might fail. The user-session agent is not elevated unless
	// the unattended (LocalSystem) service is running.
	if !isWindowsAdmin() {
		_ = m.write(hub.AgentEnvelope{Type: "terminal_data", SessionID: sessionID, Data: limitedTerminalBanner})
	}

	go m.streamOutput(sessionID, pty)
	go m.waitForExit(sessionID, processHandle)
}

func (m *windowsTerminalManager) writeInput(sessionID, data string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok || session == nil || session.pty == nil {
		return
	}

	input := normalizeTerminalInput(data)
	if input == "" {
		return
	}
	if _, err := io.WriteString(session.pty, input); err != nil && !errors.Is(err, os.ErrClosed) {
		log.Printf("terminal input write failed (%s): %v", sessionID, err)
	}
}

func (m *windowsTerminalManager) closeSession(sessionID string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	if session == nil {
		return
	}
	session.close()
}

func (m *windowsTerminalManager) resize(sessionID string, cols, rows uint16) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok || session == nil || session.pty == nil {
		return
	}
	if err := session.pty.Resize(cols, rows); err != nil && !errors.Is(err, os.ErrClosed) {
		log.Printf("terminal resize failed (%s): %v", sessionID, err)
	}
}

func (m *windowsTerminalManager) streamOutput(sessionID string, reader io.Reader) {
	buf := make([]byte, 2048)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_ = m.write(hub.AgentEnvelope{Type: "terminal_data", SessionID: sessionID, Data: string(buf[:n])})
		}
		if err != nil {
			if !isNormalTerminalReadError(err) {
				log.Printf("terminal stream read failed (%s): %v", sessionID, err)
			}
			return
		}
	}
}

func (m *windowsTerminalManager) waitForExit(sessionID string, processHandle windows.Handle) {
	exitCode := waitForWindowsProcessExitCode(processHandle)
	if exitCode < 0 {
		exitCode = 1
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	_ = m.write(hub.AgentEnvelope{Type: "terminal_exit", SessionID: sessionID, ExitCode: exitCode})
}

func (m *windowsTerminalManager) sendError(sessionID, message string) {
	_ = m.write(hub.AgentEnvelope{Type: "terminal_error", SessionID: sessionID, Error: message})
	_ = m.write(hub.AgentEnvelope{Type: "terminal_exit", SessionID: sessionID, ExitCode: 1})
}

func isWindowsAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	// IsElevated reflects the real elevation state of this process, so an admin
	// user running un-elevated under UAC correctly reports false (and gets the
	// limited-session banner), while the LocalSystem service reports true.
	return token.IsElevated()
}

func normalizeTerminalInput(data string) string {
	if data == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(data))
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' && (i == 0 || data[i-1] != '\r') {
			builder.WriteByte('\r')
			continue
		}
		builder.WriteByte(data[i])
	}
	return builder.String()
}

func isNormalTerminalReadError(err error) bool {
	return err == io.EOF || err == os.ErrClosed ||
		errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windows.ERROR_OPERATION_ABORTED)
}

func waitForWindowsProcessExitCode(handle windows.Handle) int {
	if handle == 0 || handle == windows.InvalidHandle {
		return 1
	}
	if _, err := windows.WaitForSingleObject(handle, windows.INFINITE); err != nil {
		return 1
	}
	var rawExitCode uint32
	if err := windows.GetExitCodeProcess(handle, &rawExitCode); err != nil {
		return 1
	}
	if rawExitCode > 0x7fffffff {
		return 1
	}
	if err := windows.CloseHandle(handle); err != nil {
		return int(rawExitCode)
	}
	return int(rawExitCode)
}

type windowsPseudoTerminal struct {
	hConsole   windows.Handle
	inputWrite windows.Handle
	outputRead windows.Handle
	mu         sync.Mutex
	closed     bool
}

func startWindowsTerminalProcess(cmd *exec.Cmd, cols, rows uint16) (*windowsPseudoTerminal, windows.Handle, error) {
	inputRead, inputWrite, err := createPipePair()
	if err != nil {
		return nil, 0, err
	}
	outputRead, outputWrite, err := createPipePair()
	if err != nil {
		closeHandles(inputRead, inputWrite)
		return nil, 0, err
	}

	var hConsole windows.Handle
	err = windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, inputRead, outputWrite, 0, &hConsole)
	if err != nil {
		closeHandles(inputRead, inputWrite, outputRead, outputWrite)
		return nil, 0, err
	}

	attributeList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(hConsole)
		closeHandles(inputRead, inputWrite, outputRead, outputWrite)
		return nil, 0, err
	}
	defer attributeList.Delete()
	if err := attributeList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(hConsole), unsafe.Sizeof(hConsole)); err != nil {
		windows.ClosePseudoConsole(hConsole)
		closeHandles(inputRead, inputWrite, outputRead, outputWrite)
		return nil, 0, err
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// STARTF_USESTDHANDLES with NULL std handles is essential: it stops the child
	// from inheriting THIS process's standard handles. The agent is launched with
	// its stdout/stderr redirected to log files, and without this flag cmd.exe
	// inherits that log-file handle as its stdout, so all shell output (banner,
	// prompt, command results) is written to the agent log instead of the
	// pseudoconsole. Leaving the handles as 0 lets the ConPTY attribute bind the
	// child's console to the pseudoconsole screen buffer.
	startupInfo := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:        uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			Flags:     windows.STARTF_USESTDHANDLES,
			StdInput:  0,
			StdOutput: 0,
			StdErr:    0,
		},
		ProcThreadAttributeList: attributeList.List(),
	}

	// For a pseudoconsole the child must attach to the ConPTY, so we must not
	// pass CREATE_NO_WINDOW / CREATE_NEW_CONSOLE (which would give it a separate
	// console and route its output away from our pipe). EXTENDED_STARTUPINFO_PRESENT
	// plus the pseudoconsole attribute is all that is required; the ConPTY has no
	// visible window of its own.
	creationFlags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT)

	commandLine := windows.ComposeCommandLine(cmd.Args)
	if cmd.SysProcAttr.CmdLine != "" {
		commandLine = cmd.SysProcAttr.CmdLine
	}
	commandLinePtr, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		windows.ClosePseudoConsole(hConsole)
		closeHandles(inputRead, inputWrite, outputRead, outputWrite)
		return nil, 0, err
	}

	var workingDirPtr *uint16
	if cmd.Dir != "" {
		workingDirPtr, err = windows.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			windows.ClosePseudoConsole(hConsole)
			closeHandles(inputRead, inputWrite, outputRead, outputWrite)
			return nil, 0, err
		}
	}

	var processInfo windows.ProcessInformation
	if err := windows.CreateProcess(nil, commandLinePtr, nil, nil, false, creationFlags, nil, workingDirPtr, &startupInfo.StartupInfo, &processInfo); err != nil {
		windows.ClosePseudoConsole(hConsole)
		closeHandles(inputRead, inputWrite, outputRead, outputWrite)
		return nil, 0, err
	}
	_ = windows.CloseHandle(processInfo.Thread)
	closeHandles(inputRead, outputWrite)

	return &windowsPseudoTerminal{
		hConsole:   hConsole,
		inputWrite: inputWrite,
		outputRead: outputRead,
	}, processInfo.Process, nil
}

func createPipePair() (windows.Handle, windows.Handle, error) {
	var readHandle, writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, nil, 0); err != nil {
		return 0, 0, err
	}
	return readHandle, writeHandle, nil
}

func closeHandles(handles ...windows.Handle) error {
	var closeErr error
	for _, handle := range handles {
		if handle == 0 || handle == windows.InvalidHandle {
			continue
		}
		if err := windows.CloseHandle(handle); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

func (pty *windowsPseudoTerminal) Read(p []byte) (int, error) {
	pty.mu.Lock()
	if pty.closed || pty.outputRead == 0 || pty.outputRead == windows.InvalidHandle {
		pty.mu.Unlock()
		return 0, os.ErrClosed
	}
	handle := pty.outputRead
	pty.mu.Unlock()

	// The blocking ReadFile must not hold pty.mu, otherwise Close (invoked from
	// the agent read loop when a terminal_close arrives) would deadlock waiting
	// for this reader, freezing the entire agent read loop.
	var bytesRead uint32
	err := windows.ReadFile(handle, p, &bytesRead, nil)
	return int(bytesRead), err
}

func (pty *windowsPseudoTerminal) Write(p []byte) (int, error) {
	pty.mu.Lock()
	if pty.closed || pty.inputWrite == 0 || pty.inputWrite == windows.InvalidHandle {
		pty.mu.Unlock()
		return 0, os.ErrClosed
	}
	handle := pty.inputWrite
	pty.mu.Unlock()

	var bytesWritten uint32
	err := windows.WriteFile(handle, p, &bytesWritten, nil)
	return int(bytesWritten), err
}

func (pty *windowsPseudoTerminal) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return os.ErrInvalid
	}
	pty.mu.Lock()
	defer pty.mu.Unlock()
	if pty.closed || pty.hConsole == 0 || pty.hConsole == windows.InvalidHandle {
		return os.ErrClosed
	}
	return windows.ResizePseudoConsole(pty.hConsole, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (pty *windowsPseudoTerminal) Close() error {
	pty.mu.Lock()
	if pty.closed {
		pty.mu.Unlock()
		return nil
	}
	pty.closed = true
	hConsole := pty.hConsole
	inputWrite := pty.inputWrite
	outputRead := pty.outputRead
	pty.hConsole = windows.InvalidHandle
	pty.inputWrite = windows.InvalidHandle
	pty.outputRead = windows.InvalidHandle
	pty.mu.Unlock()

	// Cancel any in-flight blocking ReadFile/WriteFile issued by the stream
	// goroutine so it unblocks promptly, then tear down the pseudoconsole and
	// pipe handles. None of this holds pty.mu, so the agent read loop is never
	// blocked by terminal teardown.
	if outputRead != 0 && outputRead != windows.InvalidHandle {
		_ = windows.CancelIoEx(outputRead, nil)
	}
	if inputWrite != 0 && inputWrite != windows.InvalidHandle {
		_ = windows.CancelIoEx(inputWrite, nil)
	}
	if hConsole != 0 && hConsole != windows.InvalidHandle {
		windows.ClosePseudoConsole(hConsole)
	}
	return closeHandles(inputWrite, outputRead)
}

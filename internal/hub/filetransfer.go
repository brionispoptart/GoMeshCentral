package hub

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// FileEntry describes one entry returned by a remote directory listing.
type FileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// ErrFileTransferTimeout is returned when an agent does not respond to a file
// operation within the deadline (e.g. device went offline mid-transfer).
var ErrFileTransferTimeout = errors.New("file transfer timed out")

const maxFileTransferBytes = 100 * 1024 * 1024 // 100MB safety cap; matches agent-side limit.

func (h *Hub) registerFileSession(sessionID string) chan AgentEnvelope {
	ch := make(chan AgentEnvelope, 64)
	h.mu.Lock()
	h.fileSessions[sessionID] = ch
	h.mu.Unlock()
	return ch
}

func (h *Hub) unregisterFileSession(sessionID string) {
	h.mu.Lock()
	delete(h.fileSessions, sessionID)
	h.mu.Unlock()
}

// routeFileMessage forwards an agent's file_* reply to the goroutine blocked
// waiting on that session (ListFiles/DownloadFile/UploadFile below).
func (h *Hub) routeFileMessage(msg AgentEnvelope) {
	h.mu.RLock()
	ch, ok := h.fileSessions[msg.SessionID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

// ListFiles requests a directory listing from a device and blocks for the
// agent's reply, matching the request/response shape callers expect from a
// normal HTTP handler despite the underlying transport being async over the
// agent's persistent websocket.
func (h *Hub) ListFiles(deviceID, path string) ([]FileEntry, error) {
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, err
	}
	ch := h.registerFileSession(sessionID)
	defer h.unregisterFileSession(sessionID)

	if err := h.writeAgent(deviceID, AgentEnvelope{Type: "file_list", SessionID: sessionID, Path: path}); err != nil {
		return nil, err
	}

	select {
	case msg := <-ch:
		if msg.Error != "" {
			return nil, errors.New(msg.Error)
		}
		var entries []FileEntry
		if err := json.Unmarshal([]byte(msg.Data), &entries); err != nil {
			return nil, err
		}
		return entries, nil
	case <-time.After(15 * time.Second):
		return nil, ErrFileTransferTimeout
	}
}

// DownloadFile requests file content from a device, reassembling the
// base64-encoded chunk stream into a single buffer capped at
// maxFileTransferBytes.
func (h *Hub) DownloadFile(deviceID, path string) ([]byte, string, error) {
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, "", err
	}
	ch := h.registerFileSession(sessionID)
	defer h.unregisterFileSession(sessionID)

	if err := h.writeAgent(deviceID, AgentEnvelope{Type: "file_download_start", SessionID: sessionID, Path: path}); err != nil {
		return nil, "", err
	}

	var buf []byte
	filename := ""
	for {
		select {
		case msg := <-ch:
			switch msg.Type {
			case "file_download_chunk":
				decoded, decErr := base64.StdEncoding.DecodeString(msg.Data)
				if decErr != nil {
					return nil, "", decErr
				}
				buf = append(buf, decoded...)
				if len(buf) > maxFileTransferBytes {
					return nil, "", errors.New("file exceeds 100MB transfer limit")
				}
			case "file_download_complete":
				if msg.Error != "" {
					return nil, "", errors.New(msg.Error)
				}
				if msg.Path != "" {
					filename = msg.Path
				}
				return buf, filename, nil
			}
		case <-time.After(60 * time.Second):
			return nil, "", ErrFileTransferTimeout
		}
	}
}

// UploadFile pushes file content to a device by chunking it over the same
// envelope channel used for downloads, then waits for the agent's ack.
func (h *Hub) UploadFile(deviceID, destPath string, content []byte) error {
	sessionID, err := randomSessionID()
	if err != nil {
		return err
	}
	ch := h.registerFileSession(sessionID)
	defer h.unregisterFileSession(sessionID)

	if err := h.writeAgent(deviceID, AgentEnvelope{Type: "file_upload_start", SessionID: sessionID, Path: destPath}); err != nil {
		return err
	}

	const chunkSize = 256 * 1024
	for offset := 0; offset < len(content); offset += chunkSize {
		end := offset + chunkSize
		if end > len(content) {
			end = len(content)
		}
		encoded := base64.StdEncoding.EncodeToString(content[offset:end])
		if err := h.writeAgent(deviceID, AgentEnvelope{Type: "file_upload_chunk", SessionID: sessionID, Data: encoded}); err != nil {
			return err
		}
	}
	if err := h.writeAgent(deviceID, AgentEnvelope{Type: "file_upload_complete", SessionID: sessionID}); err != nil {
		return err
	}

	select {
	case msg := <-ch:
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
		return nil
	case <-time.After(60 * time.Second):
		return ErrFileTransferTimeout
	}
}

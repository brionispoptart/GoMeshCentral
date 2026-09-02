package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"gomeshcentral/internal/hub"
)

type remoteFileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// fileTransferManager handles remote directory listing plus file
// upload/download for the agent side of the hub's chunked file_* envelope
// protocol. It is OS-agnostic (plain os/io calls), unlike the terminal
// managers which need ConPTY/PTY per platform.
type fileTransferManager struct {
	mu      sync.Mutex
	uploads map[string]*os.File
	write   func(hub.AgentEnvelope) error
}

func newFileTransferManager(write func(hub.AgentEnvelope) error) *fileTransferManager {
	return &fileTransferManager{uploads: map[string]*os.File{}, write: write}
}

const maxFileTransferBytes = 100 * 1024 * 1024 // 100MB safety cap; matches hub-side limit.

func (f *fileTransferManager) Handle(msg hub.AgentEnvelope) {
	switch msg.Type {
	case "file_list":
		f.list(msg.SessionID, msg.Path)
	case "file_download_start":
		f.download(msg.SessionID, msg.Path)
	case "file_upload_start":
		f.uploadStart(msg.SessionID, msg.Path)
	case "file_upload_chunk":
		f.uploadChunk(msg.SessionID, msg.Data)
	case "file_upload_complete":
		f.uploadComplete(msg.SessionID)
	}
}

func defaultBrowseRoot() string {
	if runtime.GOOS == "windows" {
		return `C:\`
	}
	return "/"
}

func (f *fileTransferManager) list(sessionID, path string) {
	if path == "" {
		path = defaultBrowseRoot()
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_list_result", SessionID: sessionID, Error: err.Error()})
		return
	}

	result := make([]remoteFileEntry, 0, len(entries))
	for _, e := range entries {
		size := int64(0)
		modTime := ""
		if info, ierr := e.Info(); ierr == nil {
			size = info.Size()
			modTime = info.ModTime().UTC().Format(time.RFC3339)
		}
		result = append(result, remoteFileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size, ModTime: modTime})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	payload, err := json.Marshal(result)
	if err != nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_list_result", SessionID: sessionID, Error: err.Error()})
		return
	}
	_ = f.write(hub.AgentEnvelope{Type: "file_list_result", SessionID: sessionID, Path: path, Data: string(payload)})
}

func (f *fileTransferManager) download(sessionID, path string) {
	file, err := os.Open(path)
	if err != nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_download_complete", SessionID: sessionID, Error: err.Error()})
		return
	}
	defer file.Close()

	if info, statErr := file.Stat(); statErr == nil && info.Size() > maxFileTransferBytes {
		_ = f.write(hub.AgentEnvelope{Type: "file_download_complete", SessionID: sessionID, Error: "file exceeds 100MB transfer limit"})
		return
	}

	buf := make([]byte, 256*1024)
	for {
		n, rerr := file.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			if werr := f.write(hub.AgentEnvelope{Type: "file_download_chunk", SessionID: sessionID, Data: encoded}); werr != nil {
				log.Printf("file download chunk send failed (%s): %v", sessionID, werr)
				return
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				_ = f.write(hub.AgentEnvelope{Type: "file_download_complete", SessionID: sessionID, Error: rerr.Error()})
				return
			}
			break
		}
	}
	_ = f.write(hub.AgentEnvelope{Type: "file_download_complete", SessionID: sessionID, Path: filepath.Base(path)})
}

func (f *fileTransferManager) uploadStart(sessionID, path string) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			_ = f.write(hub.AgentEnvelope{Type: "file_upload_result", SessionID: sessionID, Error: err.Error()})
			return
		}
	}
	file, err := os.Create(path)
	if err != nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_upload_result", SessionID: sessionID, Error: err.Error()})
		return
	}
	f.mu.Lock()
	f.uploads[sessionID] = file
	f.mu.Unlock()
}

func (f *fileTransferManager) uploadChunk(sessionID, data string) {
	f.mu.Lock()
	file, ok := f.uploads[sessionID]
	f.mu.Unlock()
	if !ok || file == nil {
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("file upload chunk decode failed (%s): %v", sessionID, err)
		return
	}
	if _, err := file.Write(decoded); err != nil {
		log.Printf("file upload chunk write failed (%s): %v", sessionID, err)
	}
}

func (f *fileTransferManager) uploadComplete(sessionID string) {
	f.mu.Lock()
	file, ok := f.uploads[sessionID]
	delete(f.uploads, sessionID)
	f.mu.Unlock()
	if !ok || file == nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_upload_result", SessionID: sessionID, Error: "no active upload session"})
		return
	}
	if err := file.Close(); err != nil {
		_ = f.write(hub.AgentEnvelope{Type: "file_upload_result", SessionID: sessionID, Error: err.Error()})
		return
	}
	_ = f.write(hub.AgentEnvelope{Type: "file_upload_result", SessionID: sessionID})
}

package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"gomeshcentral/internal/hub"
	"gomeshcentral/internal/storage"
)

const maxFileUploadBytes = 100 * 1024 * 1024 // 100MB safety cap; matches hub/agent limits.

func deviceIDFromFileRoute(path, suffix string) string {
	trimmed := strings.TrimPrefix(path, "/api/devices/")
	return strings.TrimSuffix(trimmed, suffix)
}

func (s *Server) handleListRemoteFiles(w http.ResponseWriter, r *http.Request) {
	deviceID := deviceIDFromFileRoute(r.URL.Path, "/files/list")
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	path := r.URL.Query().Get("path")

	entries, err := s.hub.ListFiles(deviceID, path)
	if err != nil {
		writeFileTransferError(w, err)
		return
	}
	respondJSON(w, entries)
}

func (s *Server) handleDownloadRemoteFile(w http.ResponseWriter, r *http.Request) {
	deviceID := deviceIDFromFileRoute(r.URL.Path, "/files/download")
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path query parameter is required", http.StatusBadRequest)
		return
	}

	content, filename, err := s.hub.DownloadFile(deviceID, path)
	if err != nil {
		writeFileTransferError(w, err)
		return
	}
	if filename == "" {
		filename = filepath.Base(path)
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "file_downloaded",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "path=" + path,
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, url.PathEscape(filename)))
	_, _ = w.Write(content)
}

func (s *Server) handleUploadRemoteFile(w http.ResponseWriter, r *http.Request) {
	deviceID := deviceIDFromFileRoute(r.URL.Path, "/files/upload")
	if deviceID == "" {
		http.NotFound(w, r)
		return
	}
	claims, ok := claimsFromContext(r.Context())
	if !ok || !s.authorizeDeviceScope(w, r, claims.OrgID, deviceID) {
		return
	}
	destPath := r.URL.Query().Get("path")
	if destPath == "" {
		http.Error(w, "path query parameter is required", http.StatusBadRequest)
		return
	}

	content, err := io.ReadAll(io.LimitReader(r.Body, maxFileUploadBytes+1))
	if err != nil {
		http.Error(w, "failed to read upload body", http.StatusBadRequest)
		return
	}
	if len(content) > maxFileUploadBytes {
		http.Error(w, "file exceeds 100MB upload limit", http.StatusRequestEntityTooLarge)
		return
	}

	if err := s.hub.UploadFile(deviceID, destPath, content); err != nil {
		writeFileTransferError(w, err)
		return
	}

	s.appendAuditEvent(storage.AuditEvent{
		Action:  "file_uploaded",
		Actor:   claims.Subject,
		Target:  deviceID,
		Details: "path=" + destPath,
	})
	w.WriteHeader(http.StatusNoContent)
}

func writeFileTransferError(w http.ResponseWriter, err error) {
	if errors.Is(err, hub.ErrDeviceOffline) {
		http.Error(w, "device offline", http.StatusConflict)
		return
	}
	if errors.Is(err, hub.ErrFileTransferTimeout) {
		http.Error(w, "device did not respond in time", http.StatusGatewayTimeout)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

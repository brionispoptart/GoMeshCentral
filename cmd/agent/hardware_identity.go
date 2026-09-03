package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type hardwareIdentity struct {
	MachineID string `json:"machineId"`
	SystemID  string `json:"systemId"`
	BoardID   string `json:"boardId"`
}

func collectHardwareIdentity() hardwareIdentity {
	return hardwareIdentity{
		MachineID: hashHardwareID(readMachineID()),
		SystemID:  hashHardwareID(readSystemID()),
		BoardID:   hashHardwareID(readBoardID()),
	}
}

func (id hardwareIdentity) count() int {
	count := 0
	for _, value := range []string{id.MachineID, id.SystemID, id.BoardID} {
		if value != "" {
			count++
		}
	}
	return count
}

func hashHardwareID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "unknown" || value == "none" || value == "to be filled by o.e.m." {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

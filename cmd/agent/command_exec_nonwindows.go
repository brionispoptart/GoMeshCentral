//go:build !windows

package main

import (
	"bytes"
	"os/exec"
	"strings"
)

func executeAgentCommand(command string) commandExecutionResult {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return commandExecutionResult{Output: "", ExitCode: 0, Error: "empty command"}
	}

	if strings.EqualFold(trimmed, "ping") {
		return commandExecutionResult{Output: "pong\n", ExitCode: 0}
	}

	cmd := exec.Command("/bin/sh", "-lc", trimmed)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" && !strings.HasSuffix(output, "\n") {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return commandExecutionResult{Output: output, ExitCode: exitCode, Error: err.Error()}
	}

	return commandExecutionResult{Output: output, ExitCode: 0}
}

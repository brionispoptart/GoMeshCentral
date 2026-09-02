package main

import "gomeshcentral/internal/hub"

type terminalManager interface {
	Handle(msg hub.AgentEnvelope)
	CloseAll()
}

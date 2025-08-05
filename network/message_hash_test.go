package network

import (
	"testing"
)

func TestCommandMapping(t *testing.T) {
	InitCommandMappingFromFile("./../gen/message_command_mapping.json")
}

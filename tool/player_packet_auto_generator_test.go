package tool

import (
	"github.com/fish-tennis/gserver/game"
	"testing"
)

func TestGenerator(t *testing.T) {
	autoGenerator(new(game.Money))
}
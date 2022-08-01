package pb

import (
	"google.golang.org/protobuf/types/known/anypb"
	"testing"
)

func TestTypeUrl(t *testing.T) {
	any,_ := anypb.New(&EventPlayerLevelup{
		PlayerId: 1,
		Level: 2,
	})
	t.Logf("%v", any.TypeUrl) // type.googleapis.com/gserver.EventPlayerLevelup
}

package tool

import "testing"

func TestProtoParser(t *testing.T)  {
	ParseFiles("./../pb/*.pb.go")
}

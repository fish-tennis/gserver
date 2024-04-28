package cfg

import (
	"testing"
)

func TestCfgLoad(t *testing.T) {
	dir := "./../cfgdata/"
	LoadAllCfgs(dir)
}

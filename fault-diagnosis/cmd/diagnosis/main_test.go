package main

import (
	"flag"
	"testing"
)

func TestConfigFlagDefaultValue(t *testing.T) {
	t.Helper()

	configFlag := flag.CommandLine.Lookup("config")
	if configFlag == nil {
		t.Fatal("flag config 未定义")
	}

	const want = "./configs/fault_trees_multi_template.json"
	if configFlag.DefValue != want {
		t.Fatalf("config 默认值不匹配: got %q, want %q", configFlag.DefValue, want)
	}
}

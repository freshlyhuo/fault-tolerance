package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "fault_tree.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	return path
}

func TestLoadFaultTrees_DuplicateEventIDShouldFail(t *testing.T) {
	configPath := writeConfigFile(t, `
{
  "fault_tree_id": "TREE-DUP-EVENT",
  "top_events": [
    {
      "event_id": "EVT-001",
      "name": "TOP",
      "fault_code": "FC-001",
      "gate_type": "OR",
      "children": ["EVT-001"]
    }
  ],
  "basic_events": [
    {
      "event_id": "EVT-001",
      "name": "BASIC",
      "alert_id": "ALERT-001"
    }
  ]
}
`)

	loader := NewLoader(configPath)
	_, err := loader.LoadFaultTrees()
	if err == nil {
		t.Fatal("expected duplicate event_id to fail validation")
	}

	if !strings.Contains(err.Error(), "event_id") {
		t.Fatalf("expected error to mention event_id, got: %v", err)
	}
}

func TestLoadFaultTrees_SharedAlertIDAcrossTreesAllowed(t *testing.T) {
	configPath := writeConfigFile(t, `
{
  "fault_trees": [
    {
      "fault_tree_id": "TREE-001",
      "top_events": [
        {
          "event_id": "T1-TOP-001",
          "name": "TOP-1",
          "fault_code": "FC-001",
          "gate_type": "OR",
          "children": ["T1-EVT-001"]
        }
      ],
      "basic_events": [
        {
          "event_id": "T1-EVT-001",
          "name": "BASIC-1",
          "alert_id": "ALERT-SHARED"
        }
      ]
    },
    {
      "fault_tree_id": "TREE-002",
      "top_events": [
        {
          "event_id": "T2-TOP-001",
          "name": "TOP-2",
          "fault_code": "FC-002",
          "gate_type": "OR",
          "children": ["T2-EVT-001"]
        }
      ],
      "basic_events": [
        {
          "event_id": "T2-EVT-001",
          "name": "BASIC-2",
          "alert_id": "ALERT-SHARED"
        }
      ]
    }
  ]
}
`)

	loader := NewLoader(configPath)
	faultTrees, err := loader.LoadFaultTrees()
	if err != nil {
		t.Fatalf("expected shared alert_id across trees to be allowed, got error: %v", err)
	}

	if len(faultTrees) != 2 {
		t.Fatalf("expected 2 fault trees, got %d", len(faultTrees))
	}
}
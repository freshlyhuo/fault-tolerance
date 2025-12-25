package state

import (
	"model"
	"os"
	"testing"
	"time"
)

func TestStateManager(t *testing.T) {
	// 创建临时数据库
	dbPath := "/tmp/test_state.db"
	defer os.Remove(dbPath)
	
	// 创建状态管理器
	sm, err := NewStateManager(dbPath)
	if err != nil {
		t.Fatalf("创建状态管理器失败: %v", err)
	}
	defer sm.Close()
	
	// 测试节点指标
	t.Run("UpdateAndGetNodeMetric", func(t *testing.T) {
		nodeMetric := &NodeMetric{
			Data: &model.NodeMetrics{
				ID:               "node-001",
				Status:           "online",
				MemoryTotal:      16000000000,
				MemoryFree:       8000000000,
				DiskTotal:        100.0,
				DiskFree:         50.0,
				CPUUsage:         50.0,
				ContainerTotal:   10,
				ContainerRunning: 9,
			},
			Timestamp: time.Now().Unix(),
		}
		
		// 更新指标
		if err := sm.UpdateMetric(nodeMetric); err != nil {
			t.Errorf("更新节点指标失败: %v", err)
		}
		
		// 查询最新状态
		metric, exists := sm.GetLatestState(MetricTypeNode, "node-001")
		if !exists {
			t.Error("未找到节点指标")
		}
		
		nm, ok := metric.(*NodeMetric)
		if !ok {
			t.Error("类型断言失败")
		}
		
		if nm.Data.ID != "node-001" {
			t.Errorf("节点ID不匹配: got %s, want node-001", nm.Data.ID)
		}
	})
	
	// 测试容器指标
	t.Run("UpdateAndGetContainerMetric", func(t *testing.T) {
		containerMetric := &ContainerMetric{
			Data: &model.ContainerMetrics{
				ID:           "container-001",
				Status:       "running",
				DeployStatus: "success",
				Uptime:       3600,
				CPUUsage: model.CPUUsage{
					Total: 50.0,
				},
			},
			Timestamp: time.Now().Unix(),
		}
		
		if err := sm.UpdateMetric(containerMetric); err != nil {
			t.Errorf("更新容器指标失败: %v", err)
		}
		
		metric, exists := sm.GetLatestState(MetricTypeContainer, "container-001")
		if !exists {
			t.Error("未找到容器指标")
		}
		
		cm, ok := metric.(*ContainerMetric)
		if !ok {
			t.Error("类型断言失败")
		}
		
		if cm.Data.Status != "running" {
			t.Errorf("容器状态不匹配: got %s, want running", cm.Data.Status)
		}
	})
	
	// 测试历史查询
	t.Run("QueryHistory", func(t *testing.T) {
		// 插入多条历史数据
		for i := 0; i < 10; i++ {
			nodeMetric := &NodeMetric{
				Data: &model.NodeMetrics{
					ID:       "node-002",
					Status:   "online",
					CPUUsage: float64(50 + i),
				},
				Timestamp: time.Now().Unix(),
			}
			sm.UpdateMetric(nodeMetric)
			time.Sleep(100 * time.Millisecond)
		}
		
		// 查询最近5秒的历史
		history := sm.QueryHistory(MetricTypeNode, "node-002", 5*time.Second)
		
		if len(history) == 0 {
			t.Error("未找到历史数据")
		}
		
		t.Logf("查询到 %d 条历史记录", len(history))
	})
	
	// 测试快照保存和加载
	t.Run("SnapshotSaveAndLoad", func(t *testing.T) {
		// 保存快照
		if err := sm.SaveSnapshot(); err != nil {
			t.Errorf("保存快照失败: %v", err)
		}
		
		// 创建新的管理器并加载快照
		sm2, err := NewStateManager(dbPath)
		if err != nil {
			t.Fatalf("创建第二个管理器失败: %v", err)
		}
		defer sm2.Close()
		
		// 验证数据已恢复
		metric, exists := sm2.GetLatestState(MetricTypeNode, "node-001")
		if !exists {
			t.Error("快照恢复后未找到节点指标")
		}
		
		nm, ok := metric.(*NodeMetric)
		if !ok {
			t.Error("类型断言失败")
		}
		
		if nm.Data.ID != "node-001" {
			t.Errorf("快照恢复后节点ID不匹配: got %s, want node-001", nm.Data.ID)
		}
	})
	
	// 测试统计信息
	t.Run("GetStats", func(t *testing.T) {
		stats := sm.GetStats()
		
		if stats["latest_states"].(int) == 0 {
			t.Error("统计信息显示没有状态")
		}
		
		t.Logf("状态统计: %+v", stats)
	})
}

// BenchmarkUpdateMetric 性能测试
func BenchmarkUpdateMetric(b *testing.B) {
	dbPath := "/tmp/bench_state.db"
	defer os.Remove(dbPath)
	
	sm, _ := NewStateManager(dbPath)
	defer sm.Close()
	
	nodeMetric := &NodeMetric{
		Data: &model.NodeMetrics{
			ID:       "node-bench",
			Status:   "online",
			CPUUsage: 50.0,
		},
		Timestamp: time.Now().Unix(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.UpdateMetric(nodeMetric)
	}
}

// BenchmarkGetLatestState 查询性能测试
func BenchmarkGetLatestState(b *testing.B) {
	dbPath := "/tmp/bench_state2.db"
	defer os.Remove(dbPath)
	
	sm, _ := NewStateManager(dbPath)
	defer sm.Close()
	
	// 预先插入数据
	nodeMetric := &NodeMetric{
		Data: &model.NodeMetrics{
			ID:       "node-bench",
			Status:   "online",
			CPUUsage: 50.0,
		},
		Timestamp: time.Now().Unix(),
	}
	sm.UpdateMetric(nodeMetric)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetLatestState(MetricTypeNode, "node-bench")
	}
}

// BenchmarkRingBufferAppend Ring Buffer性能测试
func BenchmarkRingBufferAppend(b *testing.B) {
	rb := NewRingBuffer(1000)
	entry := HistoryEntry{
		Timestamp: time.Now().Unix(),
		Data:      "test data",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Append(entry)
	}
}

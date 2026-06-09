package state

import (
	model "health-monitor/pkg/models"
	"testing"
	"time"
)

func newBusinessMetric(data interface{}) *BusinessMetric {
	return &BusinessMetric{
		Data: &model.BusinessMetrics{
			Timestamp: time.Now().Unix(),
			Data:      data,
		},
		Timestamp: time.Now().Unix(),
	}
}

func TestStateManager(t *testing.T) {
	// 创建状态管理器
	sm, err := NewStateManager()
	if err != nil {
		t.Fatalf("创建状态管理器失败: %v", err)
	}
	defer sm.Close()

	// 测试业务指标
	t.Run("UpdateAndGetBusinessMetric", func(t *testing.T) {
		businessMetric := newBusinessMetric(&model.PowerMetrics{
			BatteryVoltage: 24.0,
			BusVoltage:     24.0,
		})

		// 更新指标
		if err := sm.UpdateMetric(businessMetric); err != nil {
			t.Errorf("更新业务指标失败: %v", err)
		}

		// 查询最新状态
		metric, exists := sm.GetLatestState("power")
		if !exists {
			t.Error("未找到业务指标")
		}

		bm, ok := metric.(*BusinessMetric)
		if !ok {
			t.Error("类型断言失败")
		}

		if _, ok := bm.Data.Data.(*model.PowerMetrics); !ok {
			t.Errorf("业务指标类型不匹配: got %T, want *model.PowerMetrics", bm.Data.Data)
		}
	})

	// 测试历史查询
	t.Run("QueryHistory", func(t *testing.T) {
		// 插入多条历史数据
		for i := 0; i < 10; i++ {
			businessMetric := newBusinessMetric(&model.CommMetrics{
				ReceiveCmdCount: uint32(i),
			})
			sm.UpdateMetric(businessMetric)
			time.Sleep(100 * time.Millisecond)
		}

		// 查询最近5秒的历史
		history := sm.QueryHistory("comm", 5*time.Second)

		if len(history) == 0 {
			t.Error("未找到历史数据")
		}

		t.Logf("查询到 %d 条历史记录", len(history))
	})

	// 测试快照行为（当前实现为纯内存模式，不跨实例恢复）
	t.Run("SnapshotInMemoryMode", func(t *testing.T) {
		if err := sm.SaveSnapshot(); err != nil {
			t.Errorf("保存快照失败: %v", err)
		}

		sm2, err := NewStateManager()
		if err != nil {
			t.Fatalf("创建第二个管理器失败: %v", err)
		}
		defer sm2.Close()

		if _, exists := sm2.GetLatestState("power"); exists {
			t.Error("纯内存模式下不应跨实例恢复历史状态")
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

func TestCheckAndUpdateAlertStateSkipsInitialResolved(t *testing.T) {
	sm, err := NewStateManager()
	if err != nil {
		t.Fatalf("创建状态管理器失败: %v", err)
	}
	defer sm.Close()

	shouldSend, firing := sm.CheckAndUpdateAlertState("ALERT-A", false)
	if shouldSend || firing {
		t.Fatalf("initial normal state should not send resolved, got shouldSend=%v firing=%v", shouldSend, firing)
	}

	shouldSend, firing = sm.CheckAndUpdateAlertState("ALERT-A", true)
	if !shouldSend || !firing {
		t.Fatalf("normal->firing should send firing, got shouldSend=%v firing=%v", shouldSend, firing)
	}

	shouldSend, firing = sm.CheckAndUpdateAlertState("ALERT-A", false)
	if !shouldSend || firing {
		t.Fatalf("firing->normal should send resolved, got shouldSend=%v firing=%v", shouldSend, firing)
	}
}

// BenchmarkUpdateMetric 性能测试
func BenchmarkUpdateMetric(b *testing.B) {
	sm, _ := NewStateManager()
	defer sm.Close()

	businessMetric := newBusinessMetric(&model.PowerMetrics{BatteryVoltage: 24.0})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.UpdateMetric(businessMetric)
	}
}

// BenchmarkGetLatestState 查询性能测试
func BenchmarkGetLatestState(b *testing.B) {
	sm, _ := NewStateManager()
	defer sm.Close()

	// 预先插入数据
	businessMetric := newBusinessMetric(&model.PowerMetrics{BatteryVoltage: 24.0})
	sm.UpdateMetric(businessMetric)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetLatestState("power")
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

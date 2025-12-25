/*将业务指标 push 到 state manager,统一通路：StateManager.UpdateMetric()
*/
package business

import (
	"context"
	"fmt"
	"time"

	"health-monitor/pkg/alert"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

// Dispatcher 业务层分发器
type Dispatcher struct {
	generator    *alert.Generator
	stateManager *state.StateManager
}

// NewDispatcher 创建新的分发器
func NewDispatcher(stateManager *state.StateManager) *Dispatcher {
	return &Dispatcher{
		generator:    alert.NewGeneratorWithStateManager(stateManager), // 使用带状态管理的生成器
		stateManager: stateManager,
	}
}

// HandleBusinessMetrics 处理业务层解析后的指标
func (d *Dispatcher) HandleBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
	fmt.Printf("[业务层Dispatcher] 收到解析指标：Comp=0x%02X Timestamp=%d\n", 
		bm.ComponentType, bm.Timestamp)
	
	// 1. 推送到 StateManager
	if d.stateManager != nil {
		businessMetric := &state.BusinessMetric{
			Data:      bm,
			Timestamp: time.Now().Unix(),
		}
		if err := d.stateManager.UpdateMetric(businessMetric); err != nil {
			fmt.Printf("[业务层Dispatcher] 保存到StateManager失败: %v\n", err)
		} else {
			fmt.Printf("[业务层Dispatcher] 已保存到StateManager: Component=0x%02X\n", bm.ComponentType)
		}
	}
	
	// 2. 发送到告警生成器进行阈值判断
	// Generator会调用threshold检查，生成告警事件并直接输出
	d.generator.ProcessBusinessMetrics(ctx, bm)
	
	// 3. 健康分计算
	// TODO: 实现健康分计算逻辑
	
	// 4. 写入 DB / MQ
	// TODO: 持久化指标数据
	
	// 5. 推送到可视化平台
	// TODO: 实现可视化推送
	
	// 6. 与微服务层指标融合
	// TODO: 实现指标融合逻辑
}

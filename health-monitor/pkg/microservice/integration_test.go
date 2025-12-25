package microservice

import (
	"context"
	"testing"
	"alert"
	"model"
)

// TestMicroserviceAlertIntegration 测试微服务层告警集成
func TestMicroserviceAlertIntegration(t *testing.T) {
	ctx := context.Background()
	
	// 创建告警生成器
	generator := alert.NewGenerator()
	
	// 模拟异常的微服务指标集
	metricsSet := &model.MicroServiceMetricsSet{
		NodeMetrics: []model.NodeMetrics{
			{
				ID:               "node-001",
				Status:           "online",
				MemoryTotal:      16000000000, // 16GB
				MemoryFree:       800000000,   // 0.8GB (内存使用率95%)
				DiskTotal:        100.0,
				DiskFree:         5.0,         // 磁盘使用率95%
				CPUUsage:         88.5,        // CPU使用率88.5%
				ProcessCount:     150,
				ContainerTotal:   10,
				ContainerRunning: 6,           // 容器运行比例60%
			},
		},
		ContainerMetrics: []model.ContainerMetrics{
			{
				ID:           "container-001",
				Status:       "exited",      // 容器已退出
				DeployStatus: "success",
				Uptime:       30,            // 运行时间30s
				RestartCount: 5,
				CPUUsage: model.CPUUsage{
					Total: 95.0,             // CPU使用率95%
				},
				MemoryLimit: 1000000000,     // 1GB
				MemoryUsage: 950000000,      // 0.95GB (内存使用率95%)
				SizeLimit:   10000000000,    // 10GB
				SizeUsage:   9500000000,     // 9.5GB (磁盘使用率95%)
			},
			{
				ID:           "container-002",
				Status:       "running",
				DeployStatus: "failure",     // 部署失败
				Uptime:       100,
			},
		},
		ServiceMetrics: []model.ServiceMetrics{
			{
				ID:      "service-001",
				Status:  "active",
				Healthy: false,              // 服务不健康
				ContainerStatusGroup: []string{"running", "exited", "paused", "running"}, // 运行比例50%
				InstanceOnline: 2,
				InstanceActive: 1,
			},
			{
				ID:             "service-002",
				Status:         "active",
				Healthy:        true,
				InstanceOnline: 0,           // 无在线节点
			},
		},
	}
	
	// 处理指标并生成告警
	t.Log("开始处理微服务层指标并生成告警...")
	generator.ProcessMicroserviceMetrics(ctx, metricsSet)
	
	// 预期会生成多个告警:
	// 1. 节点内存使用率过高
	// 2. 节点磁盘使用率过高
	// 3. 节点CPU使用率过高
	// 4. 节点容器运行比例过低
	// 5. 容器已退出
	// 6. 容器运行时间过短
	// 7. 容器CPU使用率过高
	// 8. 容器内存使用率过高
	// 9. 容器磁盘占用率过高
	// 10. 容器部署失败
	// 11. 服务不健康
	// 12. 服务容器运行比例过低
	// 13. 服务无在线节点
	
	t.Log("告警生成完成")
}

// TestNodeThresholdChecking 测试节点阈值检查
func TestNodeThresholdChecking(t *testing.T) {
	// 测试正常节点
	normalNode := &model.NodeMetrics{
		ID:               "node-normal",
		Status:           "online",
		MemoryTotal:      16000000000,
		MemoryFree:       8000000000,  // 50%使用率
		DiskTotal:        100.0,
		DiskFree:         50.0,        // 50%使用率
		CPUUsage:         50.0,        // 50%使用率
		ContainerTotal:   10,
		ContainerRunning: 9,           // 90%运行
	}
	
	alerts := alert.CheckNodeThresholds(normalNode)
	if len(alerts) != 0 {
		t.Errorf("正常节点不应产生告警，但产生了 %d 个", len(alerts))
	}
	
	// 测试离线节点
	offlineNode := &model.NodeMetrics{
		ID:     "node-offline",
		Status: "offline",
	}
	
	alerts = alert.CheckNodeThresholds(offlineNode)
	if len(alerts) == 0 {
		t.Error("离线节点应产生告警")
	}
}

// TestContainerThresholdChecking 测试容器阈值检查
func TestContainerThresholdChecking(t *testing.T) {
	// 测试正常容器
	normalContainer := &model.ContainerMetrics{
		ID:           "container-normal",
		Status:       "running",
		DeployStatus: "success",
		Uptime:       3600,
		CPUUsage: model.CPUUsage{
			Total: 50.0,
		},
		MemoryLimit: 1000000000,
		MemoryUsage: 500000000,  // 50%使用率
		SizeLimit:   10000000000,
		SizeUsage:   5000000000, // 50%使用率
	}
	
	alerts := alert.CheckContainerThresholds(normalContainer)
	if len(alerts) != 0 {
		t.Errorf("正常容器不应产生告警，但产生了 %d 个", len(alerts))
	}
	
	// 测试异常容器
	abnormalContainer := &model.ContainerMetrics{
		ID:           "container-abnormal",
		Status:       "exited",
		DeployStatus: "failure",
		Uptime:       30,
	}
	
	alerts = alert.CheckContainerThresholds(abnormalContainer)
	if len(alerts) < 2 {
		t.Errorf("异常容器应产生至少2个告警，实际产生了 %d 个", len(alerts))
	}
}

// TestServiceThresholdChecking 测试服务阈值检查
func TestServiceThresholdChecking(t *testing.T) {
	// 测试正常服务
	normalService := &model.ServiceMetrics{
		ID:                   "service-normal",
		Status:               "active",
		Healthy:              true,
		ContainerStatusGroup: []string{"running", "running", "running", "running"},
		InstanceOnline:       3,
	}
	
	alerts := alert.CheckServiceThresholds(normalService)
	if len(alerts) != 0 {
		t.Errorf("正常服务不应产生告警，但产生了 %d 个", len(alerts))
	}
	
	// 测试不健康服务
	unhealthyService := &model.ServiceMetrics{
		ID:      "service-unhealthy",
		Status:  "active",
		Healthy: false,
	}
	
	alerts = alert.CheckServiceThresholds(unhealthyService)
	if len(alerts) == 0 {
		t.Error("不健康服务应产生告警")
	}
}

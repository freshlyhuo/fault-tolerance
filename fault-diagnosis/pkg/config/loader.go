package config

import (
	"encoding/json"
	"fmt"
	"os"

	"fault-diagnosis/pkg/models"
)

// Loader 配置加载器
type Loader struct {
	configPath string
}

// NewLoader 创建配置加载器
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// LoadFaultTree 加载故障树配置
func (l *Loader) LoadFaultTree() (*models.FaultTree, error) {
	// 读取配置文件
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析JSON
	var faultTree models.FaultTree
	if err := json.Unmarshal(data, &faultTree); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := l.validateFaultTree(&faultTree); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &faultTree, nil
}

// validateFaultTree 验证故障树配置
func (l *Loader) validateFaultTree(ft *models.FaultTree) error {
	if ft.FaultTreeID == "" {
		return fmt.Errorf("fault_tree_id不能为空")
	}

	if len(ft.TopEvents) == 0 {
		return fmt.Errorf("至少需要一个顶层事件")
	}

	if len(ft.BasicEvents) == 0 {
		return fmt.Errorf("至少需要一个基本事件")
	}

	// 验证顶层事件
	for _, event := range ft.TopEvents {
		if event.EventID == "" {
			return fmt.Errorf("顶层事件ID不能为空")
		}
		if event.FaultCode == "" {
			return fmt.Errorf("顶层事件 %s 的故障码不能为空", event.EventID)
		}
	}

	// 验证基本事件
	basicEventIDs := make(map[string]bool)
	for _, event := range ft.BasicEvents {
		if event.EventID == "" {
			return fmt.Errorf("基本事件ID不能为空")
		}
		if event.AlertID == "" {
			return fmt.Errorf("基本事件 %s 的告警ID不能为空", event.EventID)
		}
		basicEventIDs[event.EventID] = true
	}

	return nil
}

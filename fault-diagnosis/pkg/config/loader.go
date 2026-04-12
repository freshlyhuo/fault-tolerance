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

// FaultTreeConfig 表示故障树配置文件内容（含顶层版本号）。
type FaultTreeConfig struct {
	Version   string              `json:"version"`
	FaultTrees []*models.FaultTree `json:"fault_trees"`
}

// FaultTreeCollection 多故障树配置（对象包装格式）
// 示例：{"version":"V2.1", "fault_trees": [{...}, {...}]}
type FaultTreeCollection struct {
	Version    string             `json:"version"`
	FaultTrees []models.FaultTree `json:"fault_trees"`
}

// NewLoader 创建配置加载器
func NewLoader(configPath string) *Loader {
	return &Loader{
		configPath: configPath,
	}
}

// LoadFaultTree 加载故障树配置
func (l *Loader) LoadFaultTree() (*models.FaultTree, error) {
	faultTrees, err := l.LoadFaultTrees()
	if err != nil {
		return nil, err
	}

	if len(faultTrees) != 1 {
		return nil, fmt.Errorf("当前入口仅支持单棵故障树，配置文件中检测到 %d 棵；请改用 LoadFaultTrees 或拆分配置文件", len(faultTrees))
	}

	return faultTrees[0], nil
}

// LoadFaultTreeConfig 加载故障树配置（含顶层version）。
// 支持三种JSON格式：
// 1) 包装对象：{"version":"V2.1", "fault_trees":[{...},{...}]}
// 2) 数组：[{...}, {...}]（version为空）
// 3) 单树对象：{...}（version为空）
func (l *Loader) LoadFaultTreeConfig() (*FaultTreeConfig, error) {
	// 读取配置文件
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 先尝试包装对象格式
	var collection FaultTreeCollection
	if err := json.Unmarshal(data, &collection); err == nil && len(collection.FaultTrees) > 0 {
		result := make([]*models.FaultTree, 0, len(collection.FaultTrees))
		for i := range collection.FaultTrees {
			ft := &collection.FaultTrees[i]
			if err := l.validateFaultTree(ft); err != nil {
				return nil, fmt.Errorf("第 %d 棵故障树配置验证失败: %w", i+1, err)
			}
			result = append(result, ft)
		}
		return &FaultTreeConfig{Version: collection.Version, FaultTrees: result}, nil
	}

	// 再尝试数组格式
	var faultTreeList []models.FaultTree
	if err := json.Unmarshal(data, &faultTreeList); err == nil && len(faultTreeList) > 0 {
		result := make([]*models.FaultTree, 0, len(faultTreeList))
		for i := range faultTreeList {
			ft := &faultTreeList[i]
			if err := l.validateFaultTree(ft); err != nil {
				return nil, fmt.Errorf("第 %d 棵故障树配置验证失败: %w", i+1, err)
			}
			result = append(result, ft)
		}
		return &FaultTreeConfig{FaultTrees: result}, nil
	}

	// 最后尝试单树对象格式
	var faultTree models.FaultTree
	if err := json.Unmarshal(data, &faultTree); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := l.validateFaultTree(&faultTree); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &FaultTreeConfig{FaultTrees: []*models.FaultTree{&faultTree}}, nil
}

// LoadFaultTrees 加载一个或多个故障树配置
// 支持三种JSON格式：
// 1) 单树对象：{...}
// 2) 数组：[{...}, {...}]
// 3) 包装对象：{"fault_trees":[{...},{...}]}
func (l *Loader) LoadFaultTrees() ([]*models.FaultTree, error) {
	configData, err := l.LoadFaultTreeConfig()
	if err != nil {
		return nil, err
	}

	return configData.FaultTrees, nil
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

	seenEventIDs := make(map[string]string)

	// 验证顶层事件
	for _, event := range ft.TopEvents {
		if event.EventID == "" {
			return fmt.Errorf("顶层事件ID不能为空")
		}
		if err := validateUniqueEventID(event.EventID, seenEventIDs, "顶层事件"); err != nil {
			return err
		}
		if event.FaultCode == "" {
			return fmt.Errorf("顶层事件 %s 的故障码不能为空", event.EventID)
		}
	}

	// 验证中间事件
	for _, event := range ft.IntermediateEvents {
		if event.EventID == "" {
			return fmt.Errorf("中间事件ID不能为空")
		}
		if err := validateUniqueEventID(event.EventID, seenEventIDs, "中间事件"); err != nil {
			return err
		}
	}

	// 验证基本事件
	for _, event := range ft.BasicEvents {
		if event.EventID == "" {
			return fmt.Errorf("基本事件ID不能为空")
		}
		if err := validateUniqueEventID(event.EventID, seenEventIDs, "基本事件"); err != nil {
			return err
		}
		if event.AlertID == "" {
			return fmt.Errorf("基本事件 %s 的告警ID不能为空", event.EventID)
		}
	}

	return nil
}

func validateUniqueEventID(eventID string, seen map[string]string, currentType string) error {
	if previousType, ok := seen[eventID]; ok {
		return fmt.Errorf("event_id %s 重复：%s 与 %s", eventID, previousType, currentType)
	}

	seen[eventID] = currentType
	return nil
}

package main

import (
	"fmt"
	"os"
	"sync"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

type reloadableDiagnosisEngine struct {
	configPath string
	logger     *zap.Logger
	callback   engine.DiagnosisCallback

	mu       sync.RWMutex
	current  *engine.MultiDiagnosisEngine
	checksum uint16
}

func newReloadableDiagnosisEngine(configPath string, logger *zap.Logger, callback engine.DiagnosisCallback) (*reloadableDiagnosisEngine, error) {
	manager := &reloadableDiagnosisEngine{
		configPath: configPath,
		logger:     logger,
		callback:   callback,
	}

	checksum, err := manager.reload()
	if err != nil {
		return nil, err
	}
	manager.checksum = checksum
	return manager, nil
}

func (m *reloadableDiagnosisEngine) ProcessAlert(alert *models.AlertEvent) {
	m.mu.RLock()
	current := m.current
	defer m.mu.RUnlock()

	if current != nil {
		current.ProcessAlert(alert)
	}
}

func (m *reloadableDiagnosisEngine) Reload() error {
	_, err := m.reload()
	return err
}

func (m *reloadableDiagnosisEngine) reload() (uint16, error) {
	_, checksum, err := readConfigWithChecksum(m.configPath)
	if err != nil {
		return 0, err
	}

	loader := config.NewLoader(m.configPath)
	faultTrees, err := loader.LoadFaultTrees()
	if err != nil {
		return 0, fmt.Errorf("加载故障树配置失败: %w", err)
	}

	next, err := engine.NewMultiDiagnosisEngine(faultTrees, m.logger)
	if err != nil {
		return 0, fmt.Errorf("创建诊断引擎失败: %w", err)
	}
	next.SetCallback(m.callback)

	m.mu.Lock()
	m.current = next
	m.checksum = checksum
	m.mu.Unlock()

	return checksum, nil
}

func readConfigWithChecksum(path string) ([]byte, uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return data, crc16CCITTFalse(data), nil
}

func formatChecksum(checksum uint16) string {
	return fmt.Sprintf("%04X", checksum)
}

func crc16CCITTFalse(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

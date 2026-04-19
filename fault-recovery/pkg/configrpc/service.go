package configrpc

import (
	"encoding/json"
	"errors"

	"fault-tolerance/fault-recovery/pkg/config"
)

const (
	DefaultModuleName = "fault_tolerance/fault_recovery"

	UpdateConfigRPCPath = "/fault_tolerance/fault_recovery/update_config_rpc"
)

const (
	StatusCodeSuccess       = 0
	StatusCodeChecksumError = 1
	StatusCodeParseError    = 2
)

type UpdateConfigRequest struct {
	ModuleName string `json:"module_name"`
	Version    string `json:"version"`
	Checksum   string `json:"checksum"`
	ConfigData string `json:"config_data"`
}

type UpdateConfigResponse struct {
	StatusCode    int    `json:"status_code"`
	ActiveVersion string `json:"active_version"`
}

type recoveryPlanConfigUpdater interface {
	UpdateRecoveryPlanConfig(version, checksum, configData string) error
	GetRecoveryPlanConfigStatus() config.RecoveryPlanConfigStatus
}

type defaultRecoveryPlanConfigUpdater struct{}

func (defaultRecoveryPlanConfigUpdater) UpdateRecoveryPlanConfig(version, checksum, configData string) error {
	return config.UpdateRecoveryPlanConfig(version, checksum, configData)
}

func (defaultRecoveryPlanConfigUpdater) GetRecoveryPlanConfigStatus() config.RecoveryPlanConfigStatus {
	return config.GetRecoveryPlanConfigStatus()
}

type RuntimeConfigService struct {
	moduleName string
	updater    recoveryPlanConfigUpdater
}

func NewRuntimeConfigService() *RuntimeConfigService {
	return NewRuntimeConfigServiceWithUpdater(defaultRecoveryPlanConfigUpdater{})
}

func NewRuntimeConfigServiceWithUpdater(updater recoveryPlanConfigUpdater) *RuntimeConfigService {
	if updater == nil {
		updater = defaultRecoveryPlanConfigUpdater{}
	}
	return &RuntimeConfigService{
		moduleName: DefaultModuleName,
		updater:    updater,
	}
}

func (s *RuntimeConfigService) HandleUpdateConfigPayload(payload []byte) UpdateConfigResponse {
	current := s.updater.GetRecoveryPlanConfigStatus()
	resp := UpdateConfigResponse{
		StatusCode:    StatusCodeParseError,
		ActiveVersion: current.CurrentVersion,
	}

	var req UpdateConfigRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		resp.StatusCode = StatusCodeParseError
		return resp
	}

	if req.ModuleName != s.moduleName || req.Version == "" || req.Checksum == "" || req.ConfigData == "" {
		resp.StatusCode = StatusCodeParseError
		return resp
	}

	err := s.updater.UpdateRecoveryPlanConfig(req.Version, req.Checksum, req.ConfigData)
	switch {
	case err == nil:
		latest := s.updater.GetRecoveryPlanConfigStatus()
		resp.StatusCode = StatusCodeSuccess
		resp.ActiveVersion = latest.CurrentVersion
		return resp
	case errors.Is(err, config.ErrChecksumMismatch):
		resp.StatusCode = StatusCodeChecksumError
		return resp
	case errors.Is(err, config.ErrConfigParse):
		resp.StatusCode = StatusCodeParseError
		return resp
	default:
		resp.StatusCode = StatusCodeParseError
		return resp
	}
}

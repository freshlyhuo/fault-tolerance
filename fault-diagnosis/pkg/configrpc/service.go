package configrpc

import (
	"encoding/json"
	"errors"

	"fault-diagnosis/pkg/config"
)

const (
	DefaultModuleName = "fault_tolerance/fault_diagnosis"

	UpdateConfigRPCPath = "/fault_tolerance/fault_diagnosis/update_config_rpc"
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

type faultTreeConfigUpdater interface {
	UpdateFaultTreeConfig(version, checksum, configData string) error
	GetFaultTreeConfigStatus() config.FaultTreeConfigStatus
}

type defaultFaultTreeConfigUpdater struct{}

func (defaultFaultTreeConfigUpdater) UpdateFaultTreeConfig(version, checksum, configData string) error {
	return config.UpdateFaultTreeConfig(version, checksum, configData)
}

func (defaultFaultTreeConfigUpdater) GetFaultTreeConfigStatus() config.FaultTreeConfigStatus {
	return config.GetFaultTreeConfigStatus()
}

type RuntimeConfigService struct {
	moduleName string
	updater    faultTreeConfigUpdater
}

func NewRuntimeConfigService() *RuntimeConfigService {
	return NewRuntimeConfigServiceWithUpdater(defaultFaultTreeConfigUpdater{})
}

func NewRuntimeConfigServiceWithUpdater(updater faultTreeConfigUpdater) *RuntimeConfigService {
	if updater == nil {
		updater = defaultFaultTreeConfigUpdater{}
	}
	return &RuntimeConfigService{
		moduleName: DefaultModuleName,
		updater:    updater,
	}
}

func (s *RuntimeConfigService) HandleUpdateConfigPayload(payload []byte) UpdateConfigResponse {
	current := s.updater.GetFaultTreeConfigStatus()
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

	err := s.updater.UpdateFaultTreeConfig(req.Version, req.Checksum, req.ConfigData)
	switch {
	case err == nil:
		latest := s.updater.GetFaultTreeConfigStatus()
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

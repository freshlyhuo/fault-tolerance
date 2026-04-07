package configrpc

import (
	"encoding/json"
	"errors"

	"health-monitor/pkg/config"
)

const (
	DefaultModuleName = "fault_tolerance/health_monitor"

	UpdateConfigRPCPath = "/fault_tolerance/health_monitor/update_config_rpc"
	GetStatusRPCPath    = "/fault_tolerance/health_monitor/get_status_rpc"
)

const (
	StatusCodeSuccess       = 0
	StatusCodeChecksumError = 1
	StatusCodeParseError    = 2
	StatusCodeInternalError = 3
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

type GetStatusRequest struct {
	ModuleName string `json:"module_name,omitempty"`
}

type GetStatusResponse struct {
	CurrentVersion  string `json:"current_version"`
	CurrentChecksum string `json:"current_checksum"`
}

type RuntimeConfigService struct {
	moduleName string
}

func NewRuntimeConfigService() *RuntimeConfigService {
	return &RuntimeConfigService{moduleName: DefaultModuleName}
}

func (s *RuntimeConfigService) HandleUpdateConfigPayload(payload []byte) UpdateConfigResponse {
	current := config.GetThresholdConfigStatus()
	resp := UpdateConfigResponse{
		StatusCode:    StatusCodeInternalError,
		ActiveVersion: current.CurrentVersion,
	}

	var req UpdateConfigRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		resp.StatusCode = StatusCodeParseError
		return resp
	}

	if req.ModuleName != s.moduleName || req.Version == "" || req.Checksum == "" || req.ConfigData == "" {
		resp.StatusCode = StatusCodeInternalError
		return resp
	}

	err := config.UpdateThresholdConfig(req.Version, req.Checksum, req.ConfigData)
	switch {
	case err == nil:
		latest := config.GetThresholdConfigStatus()
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
		resp.StatusCode = StatusCodeInternalError
		return resp
	}
}

func (s *RuntimeConfigService) HandleGetStatusPayload(payload []byte) GetStatusResponse {
	// module_name is optional by spec; ignore malformed/absent payload and return current status.
	if len(payload) > 0 {
		var req GetStatusRequest
		_ = json.Unmarshal(payload, &req)
	}

	status := config.GetThresholdConfigStatus()
	return GetStatusResponse{
		CurrentVersion:  status.CurrentVersion,
		CurrentChecksum: status.CurrentChecksum,
	}
}

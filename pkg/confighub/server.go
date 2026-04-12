package confighub

import (
	"encoding/json"
	"fmt"
	"sync"

	fdconfigrpc "fault-diagnosis/pkg/configrpc"
	vsoaProtocol "github.com/acoinfo/vsoa/protocol"
	vsoaServer "github.com/acoinfo/vsoa/server"
	hmconfigrpc "health-monitor/pkg/configrpc"
)

type healthConfigService interface {
	HandleUpdateConfigPayload(payload []byte) hmconfigrpc.UpdateConfigResponse
	HandleGetStatusPayload(payload []byte) hmconfigrpc.GetStatusResponse
}

type diagnosisConfigService interface {
	HandleUpdateConfigPayload(payload []byte) fdconfigrpc.UpdateConfigResponse
}

type extraRoute struct {
	path    string
	method  vsoaProtocol.RpcMessageType
	handler func(req, res *vsoaProtocol.Message)
}

// ConfigHubServer 在同一端口聚合多个模块的配置RPC接口。
type ConfigHubServer struct {
	addr string
	hm   healthConfigService
	fd   diagnosisConfigService

	mu     sync.Mutex
	server *vsoaServer.Server
	errCh  chan error
	routes []extraRoute
}

func NewConfigHubServer(addr string) *ConfigHubServer {
	return NewConfigHubServerWithServices(addr, hmconfigrpc.NewRuntimeConfigService(), fdconfigrpc.NewRuntimeConfigService())
}

func NewConfigHubServerWithServices(addr string, hm healthConfigService, fd diagnosisConfigService) *ConfigHubServer {
	if hm == nil {
		hm = hmconfigrpc.NewRuntimeConfigService()
	}
	if fd == nil {
		fd = fdconfigrpc.NewRuntimeConfigService()
	}
	return &ConfigHubServer{
		addr:  addr,
		hm:    hm,
		fd:    fd,
		errCh: make(chan error, 1),
	}
}

// RegisterRoute 允许后续模块在同端口注册新的RPC路径。
func (s *ConfigHubServer) RegisterRoute(path string, method vsoaProtocol.RpcMessageType, handler func(req, res *vsoaProtocol.Message)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = append(s.routes, extraRoute{path: path, method: method, handler: handler})
}

func (s *ConfigHubServer) Start() error {
	if s.addr == "" {
		return fmt.Errorf("config hub address is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}

	srv := vsoaServer.NewServer("fault_tolerance_config_hub", vsoaServer.Option{})
	if err := srv.On(hmconfigrpc.UpdateConfigRPCPath, vsoaProtocol.RpcMethodSet, s.handleHealthUpdateRPC); err != nil {
		return fmt.Errorf("register health monitor update rpc failed: %w", err)
	}
	if err := srv.On(hmconfigrpc.GetStatusRPCPath, vsoaProtocol.RpcMethodGet, s.handleHealthStatusRPC); err != nil {
		return fmt.Errorf("register health monitor status rpc failed: %w", err)
	}
	if err := srv.On(fdconfigrpc.UpdateConfigRPCPath, vsoaProtocol.RpcMethodSet, s.handleDiagnosisUpdateRPC); err != nil {
		return fmt.Errorf("register fault diagnosis update rpc failed: %w", err)
	}
	for _, route := range s.routes {
		if err := srv.On(route.path, route.method, route.handler); err != nil {
			return fmt.Errorf("register extra rpc %s failed: %w", route.path, err)
		}
	}

	s.server = srv
	go func() {
		if err := srv.Serve(s.addr); err != nil {
			select {
			case s.errCh <- err:
			default:
			}
		}
	}()
	return nil
}

func (s *ConfigHubServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return nil
	}
	err := s.server.Close()
	s.server = nil
	return err
}

func (s *ConfigHubServer) Errors() <-chan error {
	return s.errCh
}

func (s *ConfigHubServer) handleHealthUpdateRPC(req, res *vsoaProtocol.Message) {
	payload := decodeRequestPayload(req)
	resp := s.hm.HandleUpdateConfigPayload(payload)
	res.Param = encodeResponsePayload(resp)
	res.Data = nil
}

func (s *ConfigHubServer) handleHealthStatusRPC(req, res *vsoaProtocol.Message) {
	payload := decodeRequestPayload(req)
	resp := s.hm.HandleGetStatusPayload(payload)
	res.Param = encodeResponsePayload(resp)
	res.Data = nil
}

func (s *ConfigHubServer) handleDiagnosisUpdateRPC(req, res *vsoaProtocol.Message) {
	payload := decodeRequestPayload(req)
	resp := s.fd.HandleUpdateConfigPayload(payload)
	res.Param = encodeResponsePayload(resp)
	res.Data = nil
}

func decodeRequestPayload(req *vsoaProtocol.Message) []byte {
	if req == nil {
		return nil
	}
	if len(req.Param) > 0 {
		return req.Param
	}
	if len(req.Data) > 0 {
		return req.Data
	}
	return nil
}

func encodeResponsePayload(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"status_code":2}`)
	}
	return b
}

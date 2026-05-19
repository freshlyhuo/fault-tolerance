package configrpc

import (
	"encoding/json"
	"fmt"
	"sync"

	vsoaProtocol "github.com/acoinfo/vsoa/protocol"
	vsoaServer "github.com/acoinfo/vsoa/server"
)

type VSOAServer struct {
	addr    string
	service *RuntimeConfigService

	mu     sync.Mutex
	server *vsoaServer.Server
	errCh  chan error
}

func NewVSOAServer(addr string, service *RuntimeConfigService) *VSOAServer {
	if service == nil {
		service = NewRuntimeConfigService()
	}
	return &VSOAServer{
		addr:    addr,
		service: service,
		errCh:   make(chan error, 1),
	}
}

func (s *VSOAServer) Start() error {
	if s.addr == "" {
		return fmt.Errorf("vsoa server address is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}

	srv := vsoaServer.NewServer("fault_recovery_config_rpc", vsoaServer.Option{})
	if err := srv.On(UpdateConfigRPCPath, vsoaProtocol.RpcMethodSet, s.handleUpdateConfigRPC); err != nil {
		return fmt.Errorf("register update_config_rpc listener failed: %w", err)
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

func (s *VSOAServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return nil
	}
	err := s.server.Close()
	s.server = nil
	return err
}

func (s *VSOAServer) Errors() <-chan error {
	return s.errCh
}

func (s *VSOAServer) handleUpdateConfigRPC(req, res *vsoaProtocol.Message) {
	payload := decodeRequestPayload(req)
	resp := s.service.HandleUpdateConfigPayload(payload)
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

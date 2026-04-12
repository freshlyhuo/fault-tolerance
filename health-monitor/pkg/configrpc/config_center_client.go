package configrpc

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

const (
	RecMicroServiceRPCPath = "/rec_microService"
)

type RecMicroServiceRequest struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Checksum    string `json:"checksum"`
}

type configCenterRPC interface {
	Connect(vsoaOrURL, addressOrURL string) (string, error)
	Call(url string, mt protocol.MessageType, flags any, req *protocol.Message) (*protocol.Message, error)
	Close() error
}

type ConfigCenterClient struct {
	addr string
	rpc  configCenterRPC

	mu        sync.Mutex
	connected bool
}

func NewConfigCenterClient(addr string) *ConfigCenterClient {
	return newConfigCenterClientWithRPC(addr, client.NewClient(client.Option{}))
}

func newConfigCenterClientWithRPC(addr string, rpc configCenterRPC) *ConfigCenterClient {
	if rpc == nil {
		rpc = client.NewClient(client.Option{})
	}
	return &ConfigCenterClient{
		addr: addr,
		rpc:  rpc,
	}
}

func (c *ConfigCenterClient) FetchMicroServiceConfig(req RecMicroServiceRequest) ([]byte, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.connectLocked(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal rec_microService request: %w", err)
	}

	message := protocol.NewMessage()
	message.Param = payload

	reply, err := c.rpc.Call(RecMicroServiceRPCPath, protocol.TypeRPC, protocol.RpcMethodSet, message)
	if err != nil {
		return nil, fmt.Errorf("call rec_microService failed: %w", err)
	}
	if reply == nil {
		return nil, fmt.Errorf("rec_microService returned nil response")
	}
	if len(reply.Param) > 0 {
		return reply.Param, nil
	}
	if len(reply.Data) > 0 {
		return reply.Data, nil
	}

	return nil, fmt.Errorf("rec_microService returned empty config payload")
}

func (c *ConfigCenterClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return c.rpc.Close()
}

func (c *ConfigCenterClient) connectLocked() error {
	if c.connected {
		return nil
	}
	if strings.TrimSpace(c.addr) == "" {
		return fmt.Errorf("config center address is empty")
	}
	if _, err := c.rpc.Connect("vsoa", c.addr); err != nil {
		return fmt.Errorf("connect config center failed: %w", err)
	}
	c.connected = true
	return nil
}

func (r RecMicroServiceRequest) validate() error {
	if strings.TrimSpace(r.ServiceName) == "" {
		return fmt.Errorf("service_name is required")
	}
	if strings.TrimSpace(r.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(r.Checksum) == "" {
		return fmt.Errorf("checksum is required")
	}
	return nil
}

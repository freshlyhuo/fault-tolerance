package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

const DefaultRepairContainerAddr = "127.0.0.1:4551"

type ContainerClient interface {
	Dispatch(ctx context.Context, instructionID string, metrics map[string]float64) (DispatchResponse, error)
}

type DispatchResponse struct {
	Command struct {
		Name string `json:"name"`
	} `json:"command"`
	Decision struct {
		Allow  bool   `json:"allow"`
		Reason string `json:"reason"`
	} `json:"decision"`
	Sent  bool `json:"sent"`
	Bytes int  `json:"bytes"`
}

type VSOAContainerClient struct {
	addr     string
	password string
	timeout  time.Duration
}

func NewVSOAContainerClientFromEnv() *VSOAContainerClient {
	addr := strings.TrimSpace(os.Getenv("FR_REPAIR_CONTAINER_ADDR"))
	if addr == "" {
		addr = DefaultRepairContainerAddr
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("FR_REPAIR_CONTAINER_TIMEOUT_MS")); raw != "" {
		var ms int
		if _, err := fmt.Sscanf(raw, "%d", &ms); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	return &VSOAContainerClient{
		addr:     addr,
		password: os.Getenv("FR_REPAIR_CONTAINER_PASSWORD"),
		timeout:  timeout,
	}
}

func NewVSOAContainerClient(addr, password string, timeout time.Duration) *VSOAContainerClient {
	if strings.TrimSpace(addr) == "" {
		addr = DefaultRepairContainerAddr
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &VSOAContainerClient{addr: addr, password: password, timeout: timeout}
}

func (c *VSOAContainerClient) Dispatch(ctx context.Context, instructionID string, metrics map[string]float64) (DispatchResponse, error) {
	if c == nil {
		return DispatchResponse{}, errors.New("container client is nil")
	}
	instructionID = strings.TrimSpace(instructionID)
	if instructionID == "" {
		return DispatchResponse{}, errors.New("instruction_id is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	vc := client.NewClient(client.Option{
		Password:       c.password,
		ConnectTimeout: c.timeout,
	})
	defer vc.Close()

	if _, err := vc.Connect("vsoa", c.addr); err != nil {
		return DispatchResponse{}, fmt.Errorf("connect repair container failed: %w", err)
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"command_name": instructionID,
		"send":         true,
		"metrics":      metrics,
	})
	if err != nil {
		return DispatchResponse{}, fmt.Errorf("marshal dispatch request failed: %w", err)
	}

	msg := protocol.NewMessage()
	msg.Param = reqBody
	reply := protocol.NewMessage()
	done := make(chan *client.Call, 1)
	call := vc.Go("/v1/dispatch", protocol.TypeRPC, protocol.RpcMethodSet, msg, reply, done)

	select {
	case <-callCtx.Done():
		_ = vc.Close()
		return DispatchResponse{}, callCtx.Err()
	case completed := <-call.Done:
		if completed.Error != nil {
			return DispatchResponse{}, completed.Error
		}
		if completed.Reply != nil {
			reply = completed.Reply
		}
	}

	var envelope struct {
		OK    bool             `json:"ok"`
		Data  DispatchResponse `json:"data"`
		Error string           `json:"error"`
	}
	body, err := responseBody(reply)
	if err != nil {
		return DispatchResponse{}, err
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return DispatchResponse{}, fmt.Errorf("decode dispatch response failed: %w", err)
	}
	if !envelope.OK {
		if envelope.Error == "" {
			envelope.Error = "repair container returned ok=false"
		}
		return DispatchResponse{}, errors.New(envelope.Error)
	}
	if !envelope.Data.Decision.Allow {
		return envelope.Data, fmt.Errorf("repair container rejected dispatch: %s", envelope.Data.Decision.Reason)
	}
	if !envelope.Data.Sent || envelope.Data.Bytes <= 0 {
		return envelope.Data, fmt.Errorf("repair container did not send frame: sent=%v bytes=%d", envelope.Data.Sent, envelope.Data.Bytes)
	}
	return envelope.Data, nil
}

func responseBody(reply *protocol.Message) ([]byte, error) {
	if reply == nil {
		return nil, errors.New("repair container returned nil reply")
	}
	if len(reply.Param) > 0 {
		return normalizeResponseBody(reply.Param), nil
	}
	if len(reply.Data) > 0 {
		return normalizeResponseBody(reply.Data), nil
	}

	status := "<nil>"
	if reply.Header != nil {
		status = reply.StatusTypeText()
		if status == "" {
			status = fmt.Sprintf("code=%d", reply.StatusType())
		}
	}
	return nil, fmt.Errorf("repair container returned empty response body: status=%s param_len=%d data_len=%d", status, len(reply.Param), len(reply.Data))
}

func normalizeResponseBody(body []byte) []byte {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 || body[0] != '"' {
		return body
	}

	var decoded string
	if err := json.Unmarshal(body, &decoded); err != nil {
		return body
	}
	return []byte(decoded)
}

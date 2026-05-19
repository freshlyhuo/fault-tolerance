package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

func main() {
	addr := flag.String("addr", envDefault("FR_REPAIR_CONTAINER_ADDR", "127.0.0.1:4551"), "repair container VSOA address")
	password := flag.String("password", os.Getenv("FR_REPAIR_CONTAINER_PASSWORD"), "repair container VSOA password")
	timeout := flag.Duration("timeout", 10*time.Second, "per RPC timeout")
	command := flag.String("command", "K50032", "dispatch command_name")
	send := flag.Bool("send", false, "actually send the frame to RS422 in /v1/dispatch")
	flag.Parse()

	vc := client.NewClient(client.Option{
		Password:       *password,
		ConnectTimeout: *timeout,
	})
	defer vc.Close()

	if _, err := vc.Connect("vsoa", *addr); err != nil {
		fmt.Fprintf(os.Stderr, "connect %s failed: %v\n", *addr, err)
		os.Exit(1)
	}

	fmt.Printf("connected: %s\n", *addr)

	hadError := false
	if err := callAndPrint(context.Background(), vc, *timeout, "/health", protocol.RpcMethodGet, nil); err != nil {
		fmt.Fprintf(os.Stderr, "probe /health failed: %v\n", err)
		hadError = true
	}
	if err := callAndPrint(context.Background(), vc, *timeout, "/v1/commands", protocol.RpcMethodGet, nil); err != nil {
		fmt.Fprintf(os.Stderr, "probe /v1/commands failed: %v\n", err)
		hadError = true
	}

	dispatchReq := map[string]interface{}{
		"command_name": *command,
		"send":         *send,
		"metrics":      map[string]float64{},
	}
	if err := callAndPrint(context.Background(), vc, *timeout, "/v1/dispatch", protocol.RpcMethodSet, dispatchReq); err != nil {
		fmt.Fprintf(os.Stderr, "probe /v1/dispatch failed: %v\n", err)
		hadError = true
	}

	if hadError {
		os.Exit(1)
	}
}

func callAndPrint(ctx context.Context, vc *client.Client, timeout time.Duration, path string, method protocol.RpcMessageType, param interface{}) error {
	fmt.Printf("\n========== %s %s ==========\n", rpcMethodText(method), path)

	msg := protocol.NewMessage()
	if param != nil {
		body, err := json.Marshal(param)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		msg.Param = body
		fmt.Printf("request_param=%s\n", body)
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan *client.Call, 1)
	call := vc.Go(path, protocol.TypeRPC, method, msg, protocol.NewMessage(), done)

	var completed *client.Call
	select {
	case <-callCtx.Done():
		_ = vc.Close()
		return callCtx.Err()
	case completed = <-call.Done:
		if completed.Error != nil {
			return completed.Error
		}
	}

	reply := completed.Reply
	if reply == nil {
		reply = protocol.NewMessage()
	}
	printReply(reply)
	if len(reply.Param) == 0 && len(reply.Data) == 0 {
		return fmt.Errorf("empty response body: status=%s", statusText(reply))
	}
	return nil
}

func printReply(reply *protocol.Message) {
	fmt.Printf("status=%s param_len=%d data_len=%d\n", statusText(reply), len(reply.Param), len(reply.Data))
	if len(reply.Param) > 0 {
		fmt.Printf("param_raw=%s\n", reply.Param)
		printJSON("param_json", reply.Param)
	}
	if len(reply.Data) > 0 {
		fmt.Printf("data_hex=%s\n", strings.ToUpper(hex.EncodeToString(reply.Data)))
		fmt.Printf("data_text=%q\n", string(reply.Data))
		printJSON("data_json", reply.Data)
	}
}

func printJSON(label string, body []byte) {
	var decoded interface{}
	if err := json.Unmarshal(normalizeResponseBody(body), &decoded); err != nil {
		return
	}
	pretty, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return
	}
	fmt.Printf("%s=%s\n", label, pretty)
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

func statusText(reply *protocol.Message) string {
	if reply == nil || reply.Header == nil {
		return "<nil>"
	}
	if text := reply.StatusTypeText(); text != "" {
		return text
	}
	return fmt.Sprintf("code=%d", reply.StatusType())
}

func rpcMethodText(method protocol.RpcMessageType) string {
	if method == protocol.RpcMethodSet {
		return "SET"
	}
	return "GET"
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"strings"

	fdconfigrpc "fault-diagnosis/pkg/configrpc"
	hmconfigrpc "health-monitor/pkg/configrpc"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

type updateRequest struct {
	ModuleName string `json:"module_name"`
	Version    string `json:"version"`
	Checksum   string `json:"checksum"`
	ConfigData string `json:"config_data"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:6551", "VSOA server address")
	target := flag.String("target", "health", "target module: health|diagnosis")
	path := flag.String("path", "", "RPC path override; default by target")
	method := flag.String("method", "set", "RPC method: set|get")

	moduleName := flag.String("module", "", "module_name override; default by target")
	version := flag.String("version", "V2.0", "version field in update request")
	configData := flag.String("config-data", "", "inline config_data json string")
	configFile := flag.String("config-file", "", "read config_data from file")

	checksumMode := flag.String("checksum-mode", "auto", "checksum mode: auto|bad|custom")
	checksum := flag.String("checksum", "", "checksum used when checksum-mode=custom")

	raw := flag.String("raw", "", "raw request payload json; if set, bypass structured request build")
	flag.Parse()

	rpcPath, rpcMethod, reqPayload, err := buildRequest(
		*target,
		*path,
		*method,
		*moduleName,
		*version,
		*configData,
		*configFile,
		*checksumMode,
		*checksum,
		*raw,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build request failed: %v\n", err)
		os.Exit(2)
	}

	cli := client.NewClient(client.Option{})
	defer cli.Close()

	if _, err := cli.Connect("vsoa", *addr); err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(3)
	}

	msg := protocol.NewMessage()
	msg.Param = reqPayload

	reply, err := cli.Call(rpcPath, protocol.TypeRPC, rpcMethod, msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rpc call failed: %v\n", err)
		os.Exit(4)
	}

	resp := []byte(nil)
	if len(reply.Param) > 0 {
		resp = reply.Param
	} else {
		resp = reply.Data
	}

	fmt.Printf("target: %s\n", *target)
	fmt.Printf("addr: %s\n", *addr)
	fmt.Printf("path: %s\n", rpcPath)
	fmt.Printf("method: %s\n", strings.ToUpper(*method))
	printJSONLike("request body", reqPayload)
	printJSONLike("response body", resp)
}

func buildRequest(
	target, path, method, moduleName, version, configData, configFile, checksumMode, checksum, raw string,
) (string, protocol.RpcMessageType, []byte, error) {
	rpcPath := strings.TrimSpace(path)
	if rpcPath == "" {
		switch strings.ToLower(strings.TrimSpace(target)) {
		case "health", "health-monitor", "hm":
			rpcPath = hmconfigrpc.UpdateConfigRPCPath
		case "diagnosis", "fault-diagnosis", "fd":
			rpcPath = fdconfigrpc.UpdateConfigRPCPath
		default:
			return "", 0, nil, fmt.Errorf("unknown target %q", target)
		}
	}

	rpcMethod := protocol.RpcMethodSet
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "set":
		rpcMethod = protocol.RpcMethodSet
	case "get":
		rpcMethod = protocol.RpcMethodGet
	default:
		return "", 0, nil, fmt.Errorf("unsupported method %q, use set|get", method)
	}

	if strings.TrimSpace(raw) != "" {
		return rpcPath, rpcMethod, []byte(raw), nil
	}

	defaultModuleName := ""
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "health", "health-monitor", "hm":
		defaultModuleName = hmconfigrpc.DefaultModuleName
	case "diagnosis", "fault-diagnosis", "fd":
		defaultModuleName = fdconfigrpc.DefaultModuleName
	}

	if strings.TrimSpace(moduleName) == "" {
		moduleName = defaultModuleName
	}

	if configFile != "" {
		b, err := os.ReadFile(configFile)
		if err != nil {
			return "", 0, nil, fmt.Errorf("read config-file failed: %w", err)
		}
		configData = string(b)
	}

	if strings.TrimSpace(configData) == "" {
		return "", 0, nil, fmt.Errorf("config-data is empty; set -config-data or -config-file, or use -raw")
	}

	finalChecksum, err := computeChecksum(checksumMode, checksum, configData)
	if err != nil {
		return "", 0, nil, err
	}

	req := updateRequest{
		ModuleName: moduleName,
		Version:    version,
		Checksum:   finalChecksum,
		ConfigData: configData,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", 0, nil, fmt.Errorf("marshal request failed: %w", err)
	}
	return rpcPath, rpcMethod, payload, nil
}

func computeChecksum(mode, customChecksum, configData string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(configData))), nil
	case "bad":
		return "BAD-CHECKSUM", nil
	case "custom":
		if strings.TrimSpace(customChecksum) == "" {
			return "", fmt.Errorf("-checksum is required when checksum-mode=custom")
		}
		return customChecksum, nil
	default:
		return "", fmt.Errorf("unknown checksum-mode %q, use auto|bad|custom", mode)
	}
}

func printJSONLike(title string, raw []byte) {
	if len(raw) == 0 {
		fmt.Printf("%s: <empty>\n", title)
		return
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Printf("%s: %s\n", title, string(raw))
		return
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: %s\n", title, string(raw))
		return
	}
	fmt.Printf("%s:\n%s\n", title, string(pretty))
}

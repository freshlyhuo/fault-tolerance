package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"os"

	"health-monitor/pkg/configrpc"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3001", "VSOA server address")
	moduleName := flag.String("module", configrpc.DefaultModuleName, "target module name")
	version := flag.String("version", "V9.9", "config version for update call")
	configData := flag.String("config-data", `{"node":{"cpu_usage_max":42.5}}`, "config JSON payload")
	flag.Parse()

	cli := client.NewClient(client.Option{})
	defer cli.Close()

	if _, err := cli.Connect("vsoa", *addr); err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}

	updateReq := configrpc.UpdateConfigRequest{
		ModuleName: *moduleName,
		Version:    *version,
		Checksum:   fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(*configData))),
		ConfigData: *configData,
	}

	updateMsg := protocol.NewMessage()
	updateMsg.Param = mustJSON(updateReq)

	updateReply, err := cli.Call(
		configrpc.UpdateConfigRPCPath,
		protocol.TypeRPC,
		protocol.RpcMethodSet,
		updateMsg,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update rpc failed: %v\n", err)
		os.Exit(1)
	}

	var updateResp configrpc.UpdateConfigResponse
	if err := json.Unmarshal(updateReply.Param, &updateResp); err != nil {
		fmt.Fprintf(os.Stderr, "decode update response failed: %v\n", err)
		os.Exit(1)
	}
	printJSON("update_config_rpc response", updateResp)

	statusMsg := protocol.NewMessage()
	statusReply, err := cli.Call(
		configrpc.GetStatusRPCPath,
		protocol.TypeRPC,
		protocol.RpcMethodGet,
		statusMsg,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status rpc failed: %v\n", err)
		os.Exit(1)
	}

	var statusResp configrpc.GetStatusResponse
	if err := json.Unmarshal(statusReply.Param, &statusResp); err != nil {
		fmt.Fprintf(os.Stderr, "decode status response failed: %v\n", err)
		os.Exit(1)
	}
	printJSON("get_status_rpc response", statusResp)
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func printJSON(title string, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: %+v\n", title, v)
		return
	}
	fmt.Printf("%s:\n%s\n", title, string(b))
}

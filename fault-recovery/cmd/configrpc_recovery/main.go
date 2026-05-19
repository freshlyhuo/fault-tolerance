package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"os"

	"fault-tolerance/fault-recovery/pkg/configrpc"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"
)

type updateDemoCase struct {
	Name               string
	Request            configrpc.UpdateConfigRequest
	ExpectedStatusCode int
}

func main() {
	addr := flag.String("addr", "127.0.0.1:3001", "VSOA server address")
	moduleName := flag.String("module", configrpc.DefaultModuleName, "target module name")
	version := flag.String("version", "V2.0", "config version for update call")
	configData := flag.String("config-data", `{"version":"v2","plans":{"RP-001":{"domain":"business","target":"satellite-service"}}}`, "config JSON payload")
	flag.Parse()

	cli := client.NewClient(client.Option{})
	defer cli.Close()

	if _, err := cli.Connect("vsoa", *addr); err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}

	demoCases := buildUpdateDemoCases(*moduleName, *version, *configData)
	for i, demoCase := range demoCases {
		fmt.Printf("\n=== Case %d: %s ===\n", i+1, demoCase.Name)
		requestBody := mustJSON(demoCase.Request)
		printJSONBytes("update_config_rpc request", requestBody)

		updateResp, responseBody, err := callUpdateConfigRPC(cli, requestBody)
		if err != nil {
			fmt.Fprintf(os.Stderr, "update rpc failed in case %q: %v\n", demoCase.Name, err)
			os.Exit(1)
		}

		printJSONBytes("update_config_rpc response", responseBody)
		fmt.Printf("expected status_code=%d, actual=%d\n", demoCase.ExpectedStatusCode, updateResp.StatusCode)
	}
}

func buildUpdateDemoCases(moduleName, baseVersion, validConfig string) []updateDemoCase {
	brokenConfig := `{"version":"v2","plans":`

	return []updateDemoCase{
		{
			Name: "status_code=0 success",
			Request: configrpc.UpdateConfigRequest{
				ModuleName: moduleName,
				Version:    baseVersion + "-ok",
				Checksum:   checksumHex(validConfig),
				ConfigData: validConfig,
			},
			ExpectedStatusCode: configrpc.StatusCodeSuccess,
		},
		{
			Name: "status_code=1 checksum mismatch",
			Request: configrpc.UpdateConfigRequest{
				ModuleName: moduleName,
				Version:    baseVersion + "-checksum",
				Checksum:   "BAD-CHECKSUM",
				ConfigData: validConfig,
			},
			ExpectedStatusCode: configrpc.StatusCodeChecksumError,
		},
		{
			Name: "status_code=2 config parse error",
			Request: configrpc.UpdateConfigRequest{
				ModuleName: moduleName,
				Version:    baseVersion + "-parse",
				Checksum:   checksumHex(brokenConfig),
				ConfigData: brokenConfig,
			},
			ExpectedStatusCode: configrpc.StatusCodeParseError,
		},
	}
}

func callUpdateConfigRPC(cli *client.Client, requestBody []byte) (configrpc.UpdateConfigResponse, []byte, error) {
	updateMsg := protocol.NewMessage()
	updateMsg.Param = requestBody

	updateReply, err := cli.Call(
		configrpc.UpdateConfigRPCPath,
		protocol.TypeRPC,
		protocol.RpcMethodSet,
		updateMsg,
	)
	if err != nil {
		return configrpc.UpdateConfigResponse{}, nil, err
	}

	var updateResp configrpc.UpdateConfigResponse
	if err := json.Unmarshal(updateReply.Param, &updateResp); err != nil {
		return configrpc.UpdateConfigResponse{}, updateReply.Param, err
	}

	return updateResp, updateReply.Param, nil
}

func checksumHex(v string) string {
	return fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(v)))
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

func printJSONBytes(title string, raw []byte) {
	if len(raw) == 0 {
		fmt.Printf("%s: <empty>\n", title)
		return
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		fmt.Printf("%s: %s\n", title, string(raw))
		return
	}
	printJSON(title, decoded)
}

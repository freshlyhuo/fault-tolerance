package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acoinfo/vsoa/protocol"
	vsoaserver "github.com/acoinfo/vsoa/server"
)

type payload struct {
	Component string                 `json:"component"`
	Timestamp int64                  `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:3002", "VSOA发布订阅测试服务监听地址")
	url := flag.String("url", "/hardware/metrics", "发布硬件指标的URL")
	interval := flag.Duration("interval", time.Second, "发布周期")
	flag.Parse()

	srv := vsoaserver.NewServer("hardware-pubsub-test-server", vsoaserver.Option{AutoAuth: true})
	var seq uint32
	if err := srv.Publish(*url, *interval, func(req *protocol.Message, _ *protocol.Message) {
		seq++
		body := samplePayload(seq)
		data, err := json.Marshal(body)
		if err != nil {
			log.Printf("marshal payload: %v", err)
			return
		}
		req.Param = nil
		req.Data = data
	}); err != nil {
		log.Fatalf("register publisher %s: %v", *url, err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(*addr)
	}()

	fmt.Printf("硬件PubSub测试服务已启动: %s URL=%s interval=%s\n", *addr, *url, interval.String())
	fmt.Println("按 Ctrl+C 停止")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		fmt.Printf("\n收到退出信号: %s\n", sig)
	case err := <-errCh:
		if err != nil {
			log.Printf("server exited: %v", err)
		}
	}

	if err := srv.Close(); err != nil {
		log.Printf("close server: %v", err)
	}
}

func samplePayload(seq uint32) payload {
	speedX := 100
	if seq%5 == 0 {
		speedX = 125
	}

	return payload{
		Component: "momentum_wheel",
		Timestamp: time.Now().Unix(),
		Values: map[string]interface{}{
			"SendCmd_K53029":                           1,
			"WheelSpeedX":                              speedX,
			"WheelSpeedY":                              100,
			"WheelSpeedZ":                              100,
			"MomentumWheel_No_telemetry_count":         seq / 7,
			"TMEZD01013_CheckErrorCount":               seq / 11,
			"TMEZD01014_FrameHeaderErrorCount":         0,
			"TMEZD01015_FrameLengthErrorCount":         0,
			"TMEZD01016_ResetCount":                    0,
			"MomentumWheel_CommandSeriaPortCount":      seq,
			"MomentumWheel_CorrectRecInstructionCount": seq,
		},
	}
}

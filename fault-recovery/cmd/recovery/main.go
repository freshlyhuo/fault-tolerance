package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"time"

	"fault-tolerance/fault-recovery/pkg/recovery"
)

func main() {
	var (
		addr      = flag.String("addr", ":8088", "http listen address")
		queueSize = flag.Int("queue", 200, "recovery queue size")
		timeoutMS = flag.Int("timeout", 8000, "action timeout in ms")
	)
	flag.Parse()

	store := recovery.NewRuntimeStore()
	stateManager := recovery.NewInMemoryStateManager()
	engine := recovery.NewEngine(stateManager, recovery.NewEngineConfig{
		QueueSize: *queueSize,
		Timeout:   time.Duration(*timeoutMS) * time.Millisecond,
	})

	// 注册故障码 → 修复动作
	engine.RegisterAction("CONTAINER-RESOURCE-001", recovery.NewCircuitBreakerAction(store))
	engine.RegisterAction("BUSINESS-IMAGE-START", recovery.NewStartContainerAction(store))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	http.HandleFunc("/diagnosis", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("read body failed"))
			return
		}
		defer r.Body.Close()

		var event recovery.DiagnosisResult
		if err := json.Unmarshal(body, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid json"))
			return
		}

		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}

		if err := engine.Submit(event); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("queue full"))
			return
		}

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("fault-recovery listening on %s", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}

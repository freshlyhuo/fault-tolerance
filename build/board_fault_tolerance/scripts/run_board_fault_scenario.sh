#!/bin/sh
set -eu

SCENARIO="${1:-power_dispatch}"
APP_DIR="${APP_DIR:-/apps/fault_tolerance}"
PUBSUB_ADDR="${PUBSUB_ADDR:-127.0.0.1:6551}"
PUBSUB_URL="${PUBSUB_URL:-/hardware/metrics}"
PUBLISH_INTERVAL="${PUBLISH_INTERVAL:-2s}"
REPAIR_ADDR="${FR_REPAIR_CONTAINER_ADDR:-127.0.0.1:4551}"
EXIT_AFTER_DIAGNOSES="${EXIT_AFTER_DIAGNOSES:-1}"
RECOVERY_DRAIN_TIMEOUT="${RECOVERY_DRAIN_TIMEOUT:-45s}"
TEST_TIMEOUT="${TEST_TIMEOUT:-120s}"
LOG_DIR="${LOG_DIR:-$APP_DIR/logs}"

cd "$APP_DIR"
mkdir -p "$LOG_DIR"

export FR_REPAIR_CONTAINER_ADDR="$REPAIR_ADDR"

PUBLISHER_LOG="$LOG_DIR/board-publisher-$SCENARIO.log"
INTEGRATION_LOG="$LOG_DIR/integration-$SCENARIO.log"

cleanup() {
  if [ "${PUBLISHER_PID:-}" != "" ]; then
    kill "$PUBLISHER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "[board-test] app_dir=$APP_DIR"
echo "[board-test] scenario=$SCENARIO"
echo "[board-test] pubsub=$PUBSUB_ADDR$PUBSUB_URL repair=$REPAIR_ADDR"
echo "[board-test] publisher_log=$PUBLISHER_LOG"
echo "[board-test] integration_log=$INTEGRATION_LOG"

./board-hardware-pubsub-sylixos \
  -addr "$PUBSUB_ADDR" \
  -url "$PUBSUB_URL" \
  -interval "$PUBLISH_INTERVAL" \
  -scenario "$SCENARIO" \
  -warmup-count 3 \
  -repeat=true \
  >"$PUBLISHER_LOG" 2>&1 &
PUBLISHER_PID=$!

sleep 1

./integration-health-diagnosis-pubsub-sylixos \
  -hardware-pubsub-addr "$PUBSUB_ADDR" \
  -hardware-pubsub-url "$PUBSUB_URL" \
  -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json \
  -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json \
  -exit-after-diagnoses "$EXIT_AFTER_DIAGNOSES" \
  -recovery-drain-timeout "$RECOVERY_DRAIN_TIMEOUT" \
  -timeout "$TEST_TIMEOUT" \
  2>&1 | tee "$INTEGRATION_LOG"

echo "[board-test] completed scenario=$SCENARIO"

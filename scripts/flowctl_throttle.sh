#!/bin/sh
# 用法:
#   flowctl_throttle.sh <containerAddr> [cn] [type] [proto] [ifname] [ul] [dl] [bufs]
# 示例:
#   flowctl_throttle.sh 10.0.0.1:8080 add ip all eth0 1000 1000 0

set -e

ADDR="$1"
CN="${2:-add}"
TYPE_ARG="${3:-ip}"
PROTO="${4:-all}"
IFNAME="${5:-eth0}"
UL="${6:-0}"
DL="${7:-0}"
BUFS="${8:-0}"

if [ -z "$ADDR" ]; then
  echo "containerAddr is required" 1>&2
  exit 1
fi

HOST="${ADDR%%:*}"
PORT="${ADDR##*:}"

if [ -z "$HOST" ] || [ -z "$PORT" ] || [ "$HOST" = "$PORT" ]; then
  echo "invalid containerAddr: $ADDR" 1>&2
  exit 1
fi

# SylixOS flowctl 顺序：先上行(ul) 后下行(dl)
# flowctl <cn> <type> <ip1> <ip2> <proto> <port1> <port2> dev <ifname> <ul> <dl> <bufs>
flowctl "$CN" "$TYPE_ARG" "$HOST" "$HOST" "$PROTO" "$PORT" "$PORT" dev "$IFNAME" "$UL" "$DL" "$BUFS"

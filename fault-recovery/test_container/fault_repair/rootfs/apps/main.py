#!/usr/bin/env python3
"""
VSOA RPC server (Python version), migrated from main.go.

RPC reference:
https://docs.acoinfo.com/vsoa/basicdev/rpc/rpc_server.html
https://docs.acoinfo.com/vsoa/manual/vsoa/python.html
"""

from __future__ import annotations

import copy
import json
import logging
import os
import sys
import threading
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import vsoa


PROTOCOL_HEADER = bytes([0xC0, 0x00, 0x00, 0x09, 0x50, 0x01])
PROTOCOL_TAIL = bytes([0x00])

RAW_COMMAND_TABLE: dict[str, str] = {
    "K50166": "8008805554AA54AA",
    "K50011": "800880550AAA0AAA",
    "K50175": "8008805555AA55AA",
    "K50132": "8008805547AA47AA",
    "K50005": "8708875507AA07AA",
    "K50003": "8008805506AA06AA",
    "K50004": "8008805506550655",
    "K52519": "AF068A5581FF",
    "K55501": "95088A55810000AA",
    "K55502": "95088A55820000AA",
    "K50502": "35068A558383",
    "K50504": "35068A558585",
    "K52002": "8008805505550555",
    "K52130": "8008805546AA46AA",
    "K50164": "8008805552AA52AA",
    "K50001": "8008805505AA05AA",
    "K51001": "8408845500AA00AA",
    "K51002": "8408845500550055",
    "K51003": "8408845501AA01AA",
    "K51004": "8408845501550155",
    "K51005": "8408845502AA02AA",
    "K51006": "8408845502550255",
    "K51007": "8408845503AA03AA",
    "K51008": "8408845503550355",
    "K53029": "8408845511041104",
    "K500038":"8008805517551755",
    "K500037":"8008805517AA17AA",
    "K50032":"8008805514551450",
    "restart_ad": "8008805554AA54AA",
    "platform_heating_belt_switch_on": "800880550AAA0AAA",
    "restart_oc": "8008805555AA55AA",
    "battery_heating_belt_switch_on": "8008805547AA47AA",
    "tank_heating_belt_switch_on": "8708875507AA07AA",
    "power_communication_device": "8008805506AA06AA",
    "turn_off_power_communication_device": "8008805506550655",
    "communicator_transmission_channel_opened": "AF068A5581FF",
    "communicator_receives_attenuation_0dB": "95088A55810000AA",
    "communicator_transmit_attenuation_0dB": "95088A55820000AA",
    "communicator_telemetry_secret_mode": "35068A558383",
    "communicator_remote_secret_mode": "35068A558585",
    "gnssa_off": "8008805505550555",
    "gnssb_on": "8008805546AA46AA",
    "switch_to_gnssb": "8008805552AA52AA",
    "gnssa_power_on": "8008805505AA05AA",
    "power_gyroscope": "8408845500AA00AA",
    "power_off_gyroscope": "8408845500550055",
    "power_starsensors": "8408845502AA02AA",
    "power_off_starsensors": "8408845502550255",
    "power_momentumwheel": "8408845503AA03AA",
    "power_off_momentumwheel": "8408845503550355",
    "flywheel_test_100_revolutions_start": "8408845511041104",
    "power_mems": "8408845501AA01AA",
    "power_off_mems": "8408845501550155",
}


@dataclass(frozen=True)
class Command:
    name: str
    instruction_hex: str
    fault_code: str

    def to_dict(self) -> dict[str, str]:
        return {
            "name": self.name,
            "instruction_hex": self.instruction_hex,
            "fault_code": self.fault_code,
        }


def now_utc_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def normalize_hex(raw: str) -> str:
    normalized = "".join(raw.split()).upper()
    if not normalized:
        raise ValueError("hex string is empty")
    if len(normalized) % 2 != 0:
        raise ValueError("hex string length must be even")
    try:
        bytes.fromhex(normalized)
    except ValueError as exc:
        raise ValueError(f"invalid hex string: {exc}") from exc
    return normalized


def fnv1a64(data: bytes) -> int:
    h = 0xCBF29CE484222325
    for b in data:
        h ^= b
        h = (h * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return h


def generate_fault_code(command_hex: str) -> str:
    return f"FC{fnv1a64(command_hex.encode('utf-8')):016X}"


def build_frame(command_hex: str) -> bytes:
    command_bytes = bytes.fromhex(command_hex)
    return PROTOCOL_HEADER + command_bytes + PROTOCOL_TAIL


def write_frame(port_path: str, frame: bytes) -> int:
    fd = os.open(port_path, os.O_RDWR | os.O_SYNC)
    try:
        written = os.write(fd, frame)
        os.fsync(fd)
        return written
    finally:
        os.close(fd)


def evaluate_metrics(command_name: str, metrics: dict[str, float]) -> dict[str, Any]:
    decision: dict[str, Any] = {
        "allow": True,
        "reason": "metrics accepted",
        "metrics": metrics,
        "checked_at": now_utc_iso(),
    }

    if not metrics:
        decision["reason"] = "no metrics provided; reserved policy path"
        return decision

    reasons: list[str] = []
    if float(metrics.get("block_dispatch", 0.0)) > 0:
        decision["allow"] = False
        reasons.append("block_dispatch > 0")
    if float(metrics.get("temperature_c", 0.0)) > 85:
        decision["allow"] = False
        reasons.append("temperature_c > 85")
    if "battery_voltage_v" in metrics and float(metrics["battery_voltage_v"]) < 20:
        decision["allow"] = False
        reasons.append("battery_voltage_v < 20")
    if command_name == "flywheel_test_100_revolutions_start":
        if "flywheel_ready" in metrics and float(metrics["flywheel_ready"]) < 1:
            decision["allow"] = False
            reasons.append("flywheel_ready < 1")

    if reasons:
        decision["reason"] = "; ".join(reasons)
    return decision


def payload_param_to_dict(payload: Any, required: bool) -> dict[str, Any]:
    if payload is None:
        if required:
            raise ValueError("param is required")
        return {}

    if isinstance(payload, dict):
        param = payload.get("param", payload)
    else:
        param = getattr(payload, "param", None)

    if param is None:
        if required:
            raise ValueError("param is required")
        return {}

    if isinstance(param, (bytes, bytearray)):
        param = param.decode("utf-8")

    if isinstance(param, str):
        if not param.strip():
            if required:
                raise ValueError("param is required")
            return {}
        try:
            param = json.loads(param)
        except json.JSONDecodeError as exc:
            raise ValueError(f"decode request failed: {exc}") from exc

    if not isinstance(param, dict):
        raise ValueError("param must be a JSON object")

    return param


def normalize_metrics(raw_metrics: Any) -> dict[str, float]:
    if raw_metrics is None:
        return {}
    if not isinstance(raw_metrics, dict):
        raise ValueError("metrics must be an object")

    out: dict[str, float] = {}
    for key, value in raw_metrics.items():
        if not isinstance(key, str):
            raise ValueError("metrics key must be string")
        try:
            out[key] = float(value)
        except (TypeError, ValueError) as exc:
            raise ValueError(f"metrics[{key}] must be number") from exc
    return out


def rpc_ok(data: Any) -> dict[str, Any]:
    return {"ok": True, "data": data}


def rpc_error(err: Exception | str) -> dict[str, Any]:
    return {"ok": False, "error": str(err)}


def build_reply_payload(body: dict[str, Any]) -> dict[str, Any]:
    return {"param": body}


def request_method_is_set(request: Any, body: dict[str, Any]) -> bool:
    method = getattr(request, "method", None)
    if isinstance(method, int):
        return method == int(getattr(vsoa, "METHOD_SET", 1))
    if isinstance(method, str):
        return method.strip().upper() == "SET"

    # 兼容无 method 字段的运行环境：有 metrics 且非空时按 SET 处理。
    maybe_metrics = body.get("metrics")
    return isinstance(maybe_metrics, dict) and len(maybe_metrics) > 0


class Service:
    def __init__(self, port_path: str):
        self.port_path = port_path
        self.commands_by_name: dict[str, Command] = {}
        self.commands_by_fault_code: dict[str, Command] = {}
        self.ordered_commands: list[Command] = []

        self._metrics_lock = threading.RLock()
        self._cached_metrics: dict[str, float] = {}

        self._build_command_index()

    def _build_command_index(self) -> None:
        for name, raw_hex in RAW_COMMAND_TABLE.items():
            normalized_hex = normalize_hex(raw_hex)
            fault_code = generate_fault_code(normalized_hex)
            cmd = Command(name=name, instruction_hex=normalized_hex, fault_code=fault_code)
            self.commands_by_name[name] = cmd
            self.commands_by_fault_code.setdefault(fault_code, cmd)

        self.ordered_commands = sorted(self.commands_by_name.values(), key=lambda c: c.name)

    def snapshot_metrics(self) -> dict[str, float]:
        with self._metrics_lock:
            return copy.deepcopy(self._cached_metrics)

    def upsert_metrics(self, metrics: dict[str, float]) -> None:
        if not metrics:
            return
        with self._metrics_lock:
            self._cached_metrics.update(metrics)

    def merge_metrics(self, request_metrics: dict[str, float]) -> dict[str, float]:
        merged = self.snapshot_metrics()
        merged.update(request_metrics)
        return merged

    def resolve_command(self, command_name: str, fault_code: str) -> Command:
        cmd_name = (command_name or "").strip()
        fc = (fault_code or "").strip().upper()

        if not cmd_name and not fc:
            raise ValueError("command_name or fault_code is required")

        if cmd_name:
            cmd = self.commands_by_name.get(cmd_name)
            if cmd is None:
                raise ValueError(f"command_name not found: {cmd_name}")
            if fc and cmd.fault_code != fc:
                raise ValueError("command_name and fault_code mismatch")
            return cmd

        cmd = self.commands_by_fault_code.get(fc)
        if cmd is None:
            raise ValueError(f"fault_code not found: {fc}")
        return cmd

    def register_rpc(self, server: vsoa.Server) -> None:
        @server.command("/health")
        def rpc_health(cli: Any, request: Any, payload: Any) -> None:
            _ = payload
            self._reply(cli, request, rpc_ok({
                "status": "ok",
                "time": now_utc_iso(),
                "mode": "vsoa-rpc",
            }))

        @server.command("/v1/commands")
        def rpc_commands(cli: Any, request: Any, payload: Any) -> None:
            _ = payload
            data = {
                "count": len(self.ordered_commands),
                "commands": [c.to_dict() for c in self.ordered_commands],
                "metrics": self.snapshot_metrics(),
            }
            self._reply(cli, request, rpc_ok(data))

        @server.command("/v1/commands/by-fault")
        def rpc_command_by_fault(cli: Any, request: Any, payload: Any) -> None:
            try:
                body = payload_param_to_dict(payload, required=True)
                fault_code = str(body.get("fault_code", "")).strip().upper()
                if not fault_code:
                    raise ValueError("fault_code is required")
                cmd = self.commands_by_fault_code.get(fault_code)
                if cmd is None:
                    raise ValueError("fault_code not found")
                self._reply(cli, request, rpc_ok(cmd.to_dict()))
            except Exception as exc:  # noqa: BLE001
                self._reply(cli, request, rpc_error(exc))

        @server.command("/v1/evaluate")
        def rpc_evaluate(cli: Any, request: Any, payload: Any) -> None:
            try:
                body = payload_param_to_dict(payload, required=True)
                cmd = self.resolve_command(
                    str(body.get("command_name", "")),
                    str(body.get("fault_code", "")),
                )
                metrics = self.merge_metrics(normalize_metrics(body.get("metrics")))
                data = {
                    "command": cmd.to_dict(),
                    "decision": evaluate_metrics(cmd.name, metrics),
                }
                self._reply(cli, request, rpc_ok(data))
            except Exception as exc:  # noqa: BLE001
                self._reply(cli, request, rpc_error(exc))

        @server.command("/v1/dispatch")
        def rpc_dispatch(cli: Any, request: Any, payload: Any) -> None:
            try:
                body = payload_param_to_dict(payload, required=True)
                cmd = self.resolve_command(
                    str(body.get("command_name", "")),
                    str(body.get("fault_code", "")),
                )
                metrics = self.merge_metrics(normalize_metrics(body.get("metrics")))
                decision = evaluate_metrics(cmd.name, metrics)
                frame = build_frame(cmd.instruction_hex)

                response: dict[str, Any] = {
                    "command": cmd.to_dict(),
                    "decision": decision,
                    "frame_hex": frame.hex().upper(),
                    "sent": False,
                    "bytes": 0,
                    "port": self.port_path,
                }

                if decision["allow"] and bool(body.get("send", False)):
                    count = write_frame(self.port_path, frame)
                    response["sent"] = True
                    response["bytes"] = count

                self._reply(cli, request, rpc_ok(response))
            except Exception as exc:  # noqa: BLE001
                self._reply(cli, request, rpc_error(exc))

        @server.command("/v1/fault-codes/generate")
        def rpc_generate_fault_code(cli: Any, request: Any, payload: Any) -> None:
            try:
                body = payload_param_to_dict(payload, required=True)
                instruction_hex = str(body.get("instruction_hex", ""))
                normalized_hex = normalize_hex(instruction_hex)
                data = {
                    "instruction_hex": normalized_hex,
                    "fault_code": generate_fault_code(normalized_hex),
                }
                self._reply(cli, request, rpc_ok(data))
            except Exception as exc:  # noqa: BLE001
                self._reply(cli, request, rpc_error(exc))

        @server.command("/v1/metrics")
        def rpc_metrics(cli: Any, request: Any, payload: Any) -> None:
            try:
                body = payload_param_to_dict(payload, required=False)
                if request_method_is_set(request, body):
                    self.upsert_metrics(normalize_metrics(body.get("metrics")))
                self._reply(cli, request, rpc_ok({"metrics": self.snapshot_metrics()}))
            except Exception as exc:  # noqa: BLE001
                self._reply(cli, request, rpc_error(exc))

    @staticmethod
    def _reply(cli: Any, request: Any, body: dict[str, Any]) -> None:
        try:
            cli.reply(request.seqno, build_reply_payload(body), status=0)
        except Exception as exc:  # noqa: BLE001
            logging.exception("rpc reply failed: %s", exc)


def parse_host_port(addr: str) -> tuple[str, int]:
    text = addr.strip()
    if not text:
        return "0.0.0.0", 3001

    # 支持 host:port，按最后一个冒号切分。
    pos = text.rfind(":")
    if pos <= 0 or pos == len(text) - 1:
        raise ValueError(f"invalid VSOA_ADDR: {addr}")

    host = text[:pos]
    try:
        port = int(text[pos + 1 :])
    except ValueError as exc:
        raise ValueError(f"invalid VSOA_ADDR port: {addr}") from exc

    if port <= 0 or port > 65535:
        raise ValueError(f"invalid VSOA_ADDR port range: {addr}")
    return host, port


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
        force=True,
    )

    port_path = os.getenv("RS422_PORT", "/dev/ttyS2")
    vsoa_addr = os.getenv("VSOA_ADDR", "0.0.0.0:3001")
    vsoa_name = os.getenv("VSOA_NAME", "faultcode-vsoa-rpc")
    vsoa_password = os.getenv("VSOA_PASSWORD", "")

    logging.info("python=%s", sys.version.replace("\n", " "))
    logging.info("vsoa_module=%s", getattr(vsoa, "__file__", "<unknown>"))

    host, port = parse_host_port(vsoa_addr)
    service = Service(port_path)

    server = vsoa.Server(vsoa_name, vsoa_password)
    service.register_rpc(server)

    logging.info(
        "fault-code VSOA RPC service started on %s:%d, serial port %s",
        host,
        port,
        port_path,
    )

    ret = server.run(host, port)

    # 文档中 run 正常情况下应不返回。若返回，区分是异步实现还是启动失败。
    try:
        addr = server.address()
    except Exception as exc:  # noqa: BLE001
        raise RuntimeError(f"vsoa server exited unexpectedly, run_ret={ret!r}") from exc

    logging.warning(
        "server.run returned unexpectedly (ret=%r), but address=%s:%s is active; entering keepalive loop",
        ret,
        addr[0],
        addr[1],
    )
    while True:
        time.sleep(3600)


if __name__ == "__main__":
    main()

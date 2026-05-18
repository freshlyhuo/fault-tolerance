#!/usr/bin/env python3
from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass
from typing import Any, Callable

try:
    import vsoa  # type: ignore[import-not-found]
except Exception as exc:  # noqa: BLE001
    print(f"[FATAL] import vsoa failed: {exc}")
    sys.exit(2)


def fnv1a64(data: bytes) -> int:
    h = 0xCBF29CE484222325
    for b in data:
        h ^= b
        h = (h * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return h


def expected_fault_code(instruction_hex: str) -> str:
    return f"FC{fnv1a64(instruction_hex.encode('utf-8')):016X}"


def fmt_obj(obj: Any) -> str:
    if obj is None:
        return "None"
    try:
        return str(dict(obj))
    except Exception:  # noqa: BLE001
        return str(obj)


@dataclass
class CaseResult:
    name: str
    ok: bool
    detail: str


class RpcTester:
    def __init__(self, base: str, timeout: float) -> None:
        self.base = base.rstrip("/")
        self.timeout = timeout
        self.method_get = int(getattr(vsoa, "METHOD_GET", 0))
        self.method_set = int(getattr(vsoa, "METHOD_SET", 1))

    def call(self, path: str, method: int, param: dict[str, Any] | None = None) -> tuple[bool, str, dict[str, Any] | None]:
        payload = None if param is None else {"param": param}
        header, resp_payload, errcode = vsoa.fetch(
            self.base + path,
            method=method,
            payload=payload,
            timeout=self.timeout,
        )

        if not header:
            return False, f"no header, errcode={errcode}", None

        header_dict = dict(header)
        status = int(header_dict.get("status", 0))
        if status != 0:
            return False, f"rpc status={status}, header={header_dict}", None

        payload_dict = dict(resp_payload) if resp_payload else {}
        body = payload_dict.get("param", payload_dict)
        if not isinstance(body, dict):
            return False, f"invalid response body: {payload_dict}", None

        if body.get("ok") is not True:
            return False, f"server error: {body.get('error', '<unknown>')}", body

        return True, "ok", body.get("data")


def run_case(name: str, fn: Callable[[], tuple[bool, str]]) -> CaseResult:
    try:
        ok, detail = fn()
        return CaseResult(name=name, ok=ok, detail=detail)
    except Exception as exc:  # noqa: BLE001
        return CaseResult(name=name, ok=False, detail=f"exception: {exc}")


def main() -> int:
    parser = argparse.ArgumentParser(description="VSOA faultcode-api RPC self test")
    parser.add_argument("--host", default="127.0.0.1", help="target host")
    parser.add_argument("--port", default=4551, type=int, help="target port")
    parser.add_argument("--timeout", default=5.0, type=float, help="RPC timeout seconds")
    parser.add_argument(
        "--send-live",
        action="store_true",
        help="run live serial dispatch test with send=true",
    )
    args = parser.parse_args()

    base = f"vsoa://{args.host}:{args.port}"
    tester = RpcTester(base=base, timeout=args.timeout)

    first_command: dict[str, Any] = {}

    def case_health() -> tuple[bool, str]:
        ok, detail, data = tester.call("/health", tester.method_get)
        if not ok:
            return False, detail
        if not isinstance(data, dict) or data.get("status") != "ok":
            return False, f"unexpected health data: {data}"
        return True, "status=ok"

    def case_commands() -> tuple[bool, str]:
        ok, detail, data = tester.call("/v1/commands", tester.method_get)
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        commands = data.get("commands")
        count = data.get("count")
        if not isinstance(commands, list) or not commands:
            return False, f"commands empty or invalid: {commands}"
        if count is None:
            return False, "missing count field"
        try:
            count_num = int(count)
        except Exception:  # noqa: BLE001
            return False, f"invalid count value: {count}"
        if count_num != len(commands):
            return False, f"count mismatch, count={count}, len={len(commands)}"
        first_command.clear()
        first_command.update(commands[0])
        return True, f"count={count}, first={first_command.get('name')}"

    def case_by_fault() -> tuple[bool, str]:
        fc = str(first_command.get("fault_code", ""))
        if not fc:
            return False, "missing sample fault_code from /v1/commands"
        ok, detail, data = tester.call(
            "/v1/commands/by-fault",
            tester.method_get,
            {"fault_code": fc},
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        if data.get("fault_code") != fc:
            return False, f"fault_code mismatch: expected={fc}, got={data.get('fault_code')}"
        return True, f"fault_code={fc}"

    def case_generate_fault_code() -> tuple[bool, str]:
        instruction_hex = "8008805554AA54AA"
        ok, detail, data = tester.call(
            "/v1/fault-codes/generate",
            tester.method_set,
            {"instruction_hex": instruction_hex},
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        got = str(data.get("fault_code", ""))
        expected = expected_fault_code(instruction_hex)
        if got != expected:
            return False, f"fault_code mismatch: expected={expected}, got={got}"
        return True, f"fault_code={got}"

    def case_metrics_set() -> tuple[bool, str]:
        metrics = {"temperature_c": 30.5, "battery_voltage_v": 26.1}
        ok, detail, data = tester.call(
            "/v1/metrics",
            tester.method_set,
            {"metrics": metrics},
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        got = data.get("metrics", {})
        if not isinstance(got, dict):
            return False, f"invalid metrics response: {got}"
        if abs(float(got.get("temperature_c", -9999)) - 30.5) > 1e-6:
            return False, f"temperature_c mismatch: {got}"
        return True, f"metrics={got}"

    def case_metrics_get() -> tuple[bool, str]:
        ok, detail, data = tester.call("/v1/metrics", tester.method_get)
        if not ok:
            return False, detail
        if not isinstance(data, dict) or not isinstance(data.get("metrics"), dict):
            return False, f"invalid data: {data}"
        return True, f"metrics_keys={list(data['metrics'].keys())}"

    def case_evaluate_allow() -> tuple[bool, str]:
        ok, detail, data = tester.call(
            "/v1/evaluate",
            tester.method_set,
            {
                "command_name": "restart_ad",
                "metrics": {"temperature_c": 35.0, "battery_voltage_v": 24.0},
            },
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        decision = data.get("decision", {})
        if not isinstance(decision, dict) or decision.get("allow") is not True:
            return False, f"expected allow=true, got={decision}"
        return True, f"reason={decision.get('reason')}"

    def case_evaluate_block() -> tuple[bool, str]:
        ok, detail, data = tester.call(
            "/v1/evaluate",
            tester.method_set,
            {
                "command_name": "restart_ad",
                "metrics": {"temperature_c": 90.0, "battery_voltage_v": 24.0},
            },
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        decision = data.get("decision", {})
        if not isinstance(decision, dict) or decision.get("allow") is not False:
            return False, f"expected allow=false, got={decision}"
        reason = str(decision.get("reason", ""))
        if "temperature_c > 85" not in reason:
            return False, f"unexpected reason: {reason}"
        return True, f"reason={reason}"

    def case_dispatch_dry_run() -> tuple[bool, str]:
        ok, detail, data = tester.call(
            "/v1/dispatch",
            tester.method_set,
            {
                "command_name": "restart_ad",
                "metrics": {"temperature_c": 30.0, "battery_voltage_v": 26.0},
                "send": False,
            },
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        if data.get("sent") is not False:
            return False, f"expected sent=false, got={data.get('sent')}"
        frame_hex = str(data.get("frame_hex", ""))
        if len(frame_hex) < 2:
            return False, f"invalid frame_hex: {frame_hex}"
        return True, f"frame_hex_len={len(frame_hex)}"

    def case_dispatch_k_commands_dry_run() -> tuple[bool, str]:
        checked = []
        for command_name in ["K50166", "K500038", "K500037", "K50032"]:
            ok, detail, data = tester.call(
                "/v1/dispatch",
                tester.method_set,
                {
                    "command_name": command_name,
                    "metrics": {"temperature_c": 30.0, "battery_voltage_v": 26.0},
                    "send": False,
                },
            )
            if not ok:
                return False, f"{command_name}: {detail}"
            if not isinstance(data, dict):
                return False, f"{command_name}: invalid data: {data}"
            command = data.get("command", {})
            if not isinstance(command, dict) or command.get("name") != command_name:
                return False, f"{command_name}: unexpected command: {command}"
            checked.append(command_name)
        return True, f"checked={checked}"

    def case_dispatch_live() -> tuple[bool, str]:
        ok, detail, data = tester.call(
            "/v1/dispatch",
            tester.method_set,
            {
                "command_name": "restart_ad",
                "metrics": {"temperature_c": 30.0, "battery_voltage_v": 26.0},
                "send": True,
            },
        )
        if not ok:
            return False, detail
        if not isinstance(data, dict):
            return False, f"invalid data: {data}"
        if data.get("sent") is not True:
            return False, f"expected sent=true, got={data.get('sent')}"
        if int(data.get("bytes", 0)) <= 0:
            return False, f"expected bytes>0, got={data.get('bytes')}"
        return True, f"bytes={data.get('bytes')}"

    cases: list[tuple[str, Callable[[], tuple[bool, str]]]] = [
        ("health", case_health),
        ("commands", case_commands),
        ("command_by_fault", case_by_fault),
        ("fault_code_generate", case_generate_fault_code),
        ("metrics_set", case_metrics_set),
        ("metrics_get", case_metrics_get),
        ("evaluate_allow", case_evaluate_allow),
        ("evaluate_block", case_evaluate_block),
        ("dispatch_dry_run", case_dispatch_dry_run),
        ("dispatch_k_commands_dry_run", case_dispatch_k_commands_dry_run),
    ]
    if args.send_live:
        cases.append(("dispatch_live", case_dispatch_live))

    print(f"[INFO] target={base}, timeout={args.timeout}s, send_live={args.send_live}")
    print(f"[INFO] vsoa_module={getattr(vsoa, '__file__', '<unknown>')}")

    results: list[CaseResult] = []
    for name, fn in cases:
        result = run_case(name, fn)
        results.append(result)
        tag = "PASS" if result.ok else "FAIL"
        print(f"[{tag}] {name}: {result.detail}")

    passed = sum(1 for r in results if r.ok)
    failed = len(results) - passed
    print(f"[SUMMARY] passed={passed} failed={failed} total={len(results)}")

    if failed > 0:
        print("[RESULT] FAIL")
        return 1

    print("[RESULT] PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())

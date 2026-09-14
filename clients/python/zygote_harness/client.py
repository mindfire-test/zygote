import sys
import json
import base64
import threading
from typing import Dict, Any, Optional


class HarnessError(Exception):
    def __init__(self, code: int, message: str, data: Any = None):
        self.code = code
        self.message = message
        self.data = data
        super().__init__(f"[{code}] {message}")


class ZygoteClient:
    def __init__(self, stdin=sys.stdin, stdout=sys.stdout):
        self._stdin = stdin
        self._stdout = stdout
        self._id_counter = 0
        self._lock = threading.Lock()

    def _call(self, method: str, params: Dict[str, Any]) -> Any:
        with self._lock:
            self._id_counter += 1
            req_id = self._id_counter

            req = {
                "jsonrpc": "2.0",
                "method": method,
                "params": params,
                "id": req_id
            }

            try:
                msg = json.dumps(req)
                self._stdout.write(msg + "\n")
                self._stdout.flush()
            except Exception as e:
                raise RuntimeError(f"Failed to write to zygote harness: {e}")

            try:
                # Read response synchronously
                line = self._stdin.readline()
                if not line:
                    raise RuntimeError("Zygote harness closed the connection unexpectedly")

                res = json.loads(line)
            except Exception as e:
                raise RuntimeError(f"Failed to read from zygote harness: {e}")

            if "error" in res and res["error"] is not None:
                err = res["error"]
                raise HarnessError(err.get("code", -1), err.get("message", "Unknown error"), err.get("data"))

            if res.get("id") != req_id:
                raise RuntimeError(f"JSON-RPC ID mismatch: expected {req_id}, got {res.get('id')}")

            return res.get("result")

    def handshake(self, version: int = 1) -> int:
        res = self._call("handshake", {"version": version})
        return res.get("version", version)

    def step(self, name: str) -> Dict[str, Any]:
        return self._call("step", {"name": name})

    def effect(self, op: str, key: str, value: Optional[bytes] = None) -> bytes:
        params = {"op": op, "key": key}
        if value is not None:
            params["value"] = base64.b64encode(value).decode("utf-8")

        res = self._call("effect", params)
        b64_val = res.get("value", "")
        if b64_val:
            return base64.b64decode(b64_val)
        return b""

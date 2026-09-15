#!/bin/bash
set -e

# TTFV walkthrough script (NFR-5.4)
echo "==> Running TTFV walkthrough (NFR-5.4)..."

# 1. Compile the CLI
go build -o zyg ./cmd/zyg

# 2. Create a mock Python agent
mkdir -p ttfv_sandbox
cd ttfv_sandbox

cat << 'PY' > agent.py
import sys
import json

def read_msg():
    line = sys.stdin.readline()
    if not line: return None
    return json.loads(line)

def write_msg(m):
    sys.stdout.write(json.dumps(m) + "\n")
    sys.stdout.flush()

# Handshake (Agent initiates)
write_msg({"jsonrpc":"2.0", "id":1, "method":"handshake", "params":{"version": 1}})
read_msg()

# Simulated model call
write_msg({"jsonrpc":"2.0", "id":2, "method":"effect", "params":{"op":"model", "key":"ask", "value":"QW5zd2Vy"}})
read_msg()

# Step
write_msg({"jsonrpc":"2.0", "id":3, "method":"step", "params":{"name":"action"}})
read_msg()
PY

# 3. Record a clean run
../zyg record --agent "python3 agent.py" --dir . -o run.zip >/dev/null 2>&1

# 4. Alter the agent logic to introduce a divergence
sed 's/"key":"ask"/"key":"different_ask"/' agent.py > agent.py.tmp && mv agent.py.tmp agent.py

# 5. Replay and verify Zygote catches the divergence (expecting exit code 1)
set +e
../zyg replay run.zip --agent "python3 agent.py" > replay.log 2>&1
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -ne 1 ]; then
    echo "ERROR: Expected exit code 1 (Divergence), got $EXIT_CODE"
    cat replay.log
    exit 1
fi

if ! grep -q -E "DIVERGED|mismatch" replay.log; then
    echo "ERROR: Did not find divergence classification in output"
    cat replay.log
    exit 1
fi

echo "==> TTFV walkthrough succeeded in catching divergence!"

# Cleanup
cd ..
rm -rf ttfv_sandbox zyg

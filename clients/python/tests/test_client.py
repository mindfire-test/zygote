import unittest
import io
import json
import base64
from zygote_harness import ZygoteClient, HarnessError

class TestZygoteClient(unittest.TestCase):
    def test_effect(self):
        # Prepare a mock stdin with a JSON-RPC response
        mock_res = {
            "jsonrpc": "2.0",
            "id": 1,
            "result": {
                "value": base64.b64encode(b"ReplayedAnswer").decode("utf-8")
            }
        }
        stdin = io.StringIO(json.dumps(mock_res) + "\n")
        stdout = io.StringIO()

        client = ZygoteClient(stdin=stdin, stdout=stdout)

        # Call effect
        res_bytes = client.effect("model", "prompt-1", b"MyPrompt")

        # Verify output
        self.assertEqual(res_bytes, b"ReplayedAnswer")

        # Verify what was sent to stdout
        sent = json.loads(stdout.getvalue())
        self.assertEqual(sent["method"], "effect")
        self.assertEqual(sent["params"]["op"], "model")
        self.assertEqual(sent["params"]["key"], "prompt-1")
        self.assertEqual(base64.b64decode(sent["params"]["value"]), b"MyPrompt")

    def test_error(self):
        mock_err = {
            "jsonrpc": "2.0",
            "id": 1,
            "error": {
                "code": -32600,
                "message": "divergence: effect mismatch"
            }
        }
        stdin = io.StringIO(json.dumps(mock_err) + "\n")
        stdout = io.StringIO()

        client = ZygoteClient(stdin=stdin, stdout=stdout)

        with self.assertRaises(HarnessError) as ctx:
            client.step("step-1")

        self.assertEqual(ctx.exception.code, -32600)
        self.assertIn("divergence", ctx.exception.message)

if __name__ == '__main__':
    unittest.main()

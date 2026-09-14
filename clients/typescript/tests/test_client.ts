import * as assert from 'assert';
import { ZygoteClient, HarnessError } from '../src/client';

async function runTests() {
    // We cannot easily intercept process.stdout synchronously in a simple Node test
    // without mocking `process.stdout.write`.
    const originalWrite = process.stdout.write.bind(process.stdout);

    let sentMsg: any = null;
    (process.stdout as any).write = (chunk: string | Uint8Array, cb?: any) => {
        try {
            sentMsg = JSON.parse(chunk.toString());
        } catch (e) {}
        return true;
    };

    const client = new ZygoteClient();

    // 1. Test effect
    const effectPromise = client.effect('model', 'prompt-1', new Uint8Array([104, 101, 108, 108, 111])); // "hello"

    // Simulate Zygote response on stdin
    process.stdin.emit('data', JSON.stringify({
        jsonrpc: '2.0',
        id: 1,
        result: { value: Buffer.from('world').toString('base64') }
    }) + '\n');

    const res = await effectPromise;
    assert.strictEqual(Buffer.from(res).toString(), 'world');
    assert.strictEqual(sentMsg.method, 'effect');
    assert.strictEqual(sentMsg.params.op, 'model');
    assert.strictEqual(sentMsg.params.key, 'prompt-1');
    assert.strictEqual(sentMsg.params.value, Buffer.from('hello').toString('base64'));

    // 2. Test error
    const stepPromise = client.step('step-1');

    process.stdin.emit('data', JSON.stringify({
        jsonrpc: '2.0',
        id: 2,
        error: { code: -32600, message: 'divergence: effect mismatch' }
    }) + '\n');

    try {
        await stepPromise;
        assert.fail('Expected HarnessError');
    } catch (e: any) {
        assert.strictEqual(e.code, -32600);
        assert.ok(e.message.includes('divergence'));
    }

    client.close();

    // Restore stdout
    process.stdout.write = originalWrite;
    console.log("All TS tests passed.");
}

runTests().catch(e => {
    console.error(e);
    process.exit(1);
});

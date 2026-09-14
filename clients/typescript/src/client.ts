import * as readline from 'readline';

export class HarnessError extends Error {
    code: number;
    data?: any;

    constructor(code: number, message: string, data?: any) {
        super(`[${code}] ${message}`);
        this.name = 'HarnessError';
        this.code = code;
        this.data = data;
    }
}

export class ZygoteClient {
    private idCounter = 0;
    private pendingRequests = new Map<number, { resolve: (val: any) => void, reject: (err: any) => void }>();
    private rl: readline.Interface;

    constructor() {
        this.rl = readline.createInterface({
            input: process.stdin,
            terminal: false
        });

        this.rl.on('line', (line) => this.handleLine(line));
    }

    private handleLine(line: string) {
        if (!line.trim()) return;

        try {
            const res = JSON.parse(line);
            if (res.jsonrpc !== '2.0' || !res.id) return;

            const pending = this.pendingRequests.get(res.id);
            if (!pending) return;

            this.pendingRequests.delete(res.id);

            if (res.error) {
                pending.reject(new HarnessError(res.error.code || -1, res.error.message || 'Unknown error', res.error.data));
            } else {
                pending.resolve(res.result);
            }
        } catch (e) {
            // Unparseable lines are ignored (Standard logs)
        }
    }

    private async call(method: string, params: any): Promise<any> {
        this.idCounter++;
        const reqId = this.idCounter;

        const req = {
            jsonrpc: "2.0",
            method,
            params,
            id: reqId
        };

        return new Promise((resolve, reject) => {
            this.pendingRequests.set(reqId, { resolve, reject });

            try {
                process.stdout.write(JSON.stringify(req) + '\n');
            } catch (e) {
                this.pendingRequests.delete(reqId);
                reject(new Error(`Failed to write to zygote harness: ${e}`));
            }
        });
    }

    async handshake(version: number = 1): Promise<number> {
        const res = await this.call('handshake', { version });
        return res?.version ?? version;
    }

    async step(name: string): Promise<any> {
        return await this.call('step', { name });
    }

    async effect(op: string, key: string, value?: Uint8Array): Promise<Uint8Array> {
        const params: any = { op, key };

        if (value) {
            params.value = Buffer.from(value).toString('base64');
        }

        const res = await this.call('effect', params);
        if (res && res.value) {
            return Buffer.from(res.value, 'base64');
        }
        return new Uint8Array(0);
    }

    close() {
        this.rl.close();
    }
}

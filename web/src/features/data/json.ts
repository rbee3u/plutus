const SECRET_KEY = /(secret|password|passphrase|apikey|ciphertext|nonce)/i;

export function safeJson(value: unknown): string {
  return JSON.stringify(redact(value), null, 2);
}

function redact(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item: unknown) => redact(item));
  }
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    const record = value as Record<string, unknown>;
    for (const key of Object.keys(record)) {
      out[key] = SECRET_KEY.test(key) ? "[redacted]" : redact(record[key]);
    }
    return out;
  }
  return value;
}

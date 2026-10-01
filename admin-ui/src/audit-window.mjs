const uint = value => (typeof value === "bigint" && value >= 0n && value <= 18446744073709551615n) || (Number.isSafeInteger(value) && value >= 0);

export function validateAuditWindow(body, requestID, before = null) {
  if (!body || (requestID !== null && body.requestId !== requestID) || !Array.isArray(body.items) || typeof body.streamPresent !== "boolean" ||
      !uint(body.firstSequence) || !uint(body.lastSequence) || !Number.isInteger(body.scanned) || body.scanned < 0 || body.scanned > 256 ||
      !Number.isInteger(body.missing) || body.missing < 0 || body.missing > body.scanned || body.items.length > body.scanned - body.missing ||
      !(body.nextBefore === null || uint(body.nextBefore))) throw new Error("Invalid audit evidence window");
  if (body.nextBefore !== null && (body.scanned !== 256 || BigInt(body.nextBefore) === 0n || BigInt(body.nextBefore) <= BigInt(body.firstSequence) || BigInt(body.nextBefore) > BigInt(body.lastSequence) || (before !== null && BigInt(body.nextBefore) >= BigInt(before)))) throw new Error("Audit cursor did not progress");
  if (!body.streamPresent && (body.scanned || body.items.length || body.nextBefore !== null)) throw new Error("Absent audit Stream has evidence");
  let previous = before === null ? BigInt(body.lastSequence) + 1n : BigInt(before);
  for (const event of body.items) {
    if (!event || (requestID !== null ? event.requestId !== requestID : typeof event.requestId !== "string" || !event.requestId) || typeof event.id !== "string" || !event.id || !uint(event.sequence) ||
        BigInt(event.sequence) >= previous || BigInt(event.sequence) < BigInt(body.firstSequence) || BigInt(event.sequence) > BigInt(body.lastSequence)) throw new Error("Audit event identity or range mismatch");
    previous = BigInt(event.sequence);
  }
  return body;
}

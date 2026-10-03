// Web Crypto randomUUID() is restricted to secure contexts in browsers.
// getRandomValues() remains available to an HTTP-hosted on-premises console.
export function createRequestID(cryptoProvider=globalThis.crypto){
  if(typeof cryptoProvider?.getRandomValues!=="function")throw new Error("Secure request ID generation unavailable");
  return Array.from(cryptoProvider.getRandomValues(new Uint8Array(16)),byte=>byte.toString(16).padStart(2,"0")).join("");
}

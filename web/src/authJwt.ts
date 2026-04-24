// Minimal JWT payload decode used to read the `exp` claim from the access
// token stored in localStorage. We never verify the signature in the browser —
// that's the API's job. We only need the claim value so we can time the
// expiry warning and auto-logout.

interface JwtPayload {
  exp?: number; // seconds since unix epoch
}

function base64UrlDecode(input: string): string {
  const padded = input.replace(/-/g, '+').replace(/_/g, '/').padEnd(
    input.length + ((4 - (input.length % 4)) % 4),
    '=',
  );
  return atob(padded);
}

/**
 * Returns the token's expiry time in ms-since-epoch, or null if the token is
 * malformed, not a JWT, or carries no `exp` claim (e.g. API keys).
 */
export function getTokenExpiryMs(token: string | null | undefined): number | null {
  if (!token) return null;
  // API keys use a "ds_" prefix and aren't JWTs — they don't expire client-side.
  if (token.startsWith('ds_')) return null;
  const parts = token.split('.');
  if (parts.length !== 3) return null;
  try {
    const payload = JSON.parse(base64UrlDecode(parts[1])) as JwtPayload;
    if (typeof payload.exp !== 'number') return null;
    return payload.exp * 1000;
  } catch {
    return null;
  }
}

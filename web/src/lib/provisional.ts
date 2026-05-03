// Mints a provisional UUID with the same variant-byte invariant as Go's
// staging.NewProvisional (byte 8 has its top two bits set to 11). Backend
// staging.Service.Stage rejects any provisional_id that doesn't match this
// shape, so the dashboard owns provisional ids end-to-end.
export function newProvisionalId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  // Set version to 4 (top nibble of byte 6).
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  // Set the provisional variant: top three bits of byte 8 = 110, so the
  // canonical-string char at index 19 is always 'c' or 'd'.
  bytes[8] = (bytes[8] & 0x1f) | 0xc0;
  return formatUUID(bytes);
}

// Reports whether a UUID string carries the provisional variant byte. The
// canonical-form character at index 19 maps to the high nibble of byte 8;
// provisional means that nibble is 1100 (c) or 1101 (d).
export function isProvisionalId(id: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[cd][0-9a-f]{3}-[0-9a-f]{12}$/.test(id);
}

function formatUUID(b: Uint8Array): string {
  const hex = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}

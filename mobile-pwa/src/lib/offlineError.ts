export class OfflineWriteBlockedError extends Error {
  constructor(message = "You're offline — connect to make changes.") {
    super(message);
    this.name = 'OfflineWriteBlockedError';
  }
}

export function isOfflineWriteBlockedError(err: unknown): err is OfflineWriteBlockedError {
  return err instanceof OfflineWriteBlockedError;
}

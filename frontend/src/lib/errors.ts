export type ErrorCode =
  | 'INVALID_INPUT'
  | 'NOT_FOUND'
  | 'NOT_CONNECTED'
  | 'AUTH_FAILED'
  | 'UNREACHABLE'
  | 'TIMEOUT'
  | 'CANCELED'
  | 'READ_ONLY'
  | 'CONFLICT'
  | 'MONGO_ERROR'
  | 'UNSUPPORTED'
  | 'INTERNAL'
  | 'SECRET_BACKEND'
  | 'UNKNOWN'

export interface AppError {
  code: ErrorCode
  message: string
  detail?: string
}

/**
 * Normalizes anything thrown by a backend call. The Go side rejects promises
 * with a JSON-encoded {code, message, detail}; anything else becomes UNKNOWN.
 */
export function toAppError(err: unknown): AppError {
  if (err && typeof err === 'object' && 'code' in err && 'message' in err) {
    return err as AppError
  }
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : JSON.stringify(err)
  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed.code === 'string' && typeof parsed.message === 'string') {
      return { code: parsed.code, message: parsed.message, detail: parsed.detail || undefined }
    }
  } catch {
    // not JSON
  }
  return { code: 'UNKNOWN', message: raw || 'Unknown error' }
}

export function isCanceled(err: unknown): boolean {
  return toAppError(err).code === 'CANCELED'
}

/** Short title shown above an error message. */
export function errorTitle(e: AppError): string {
  switch (e.code) {
    case 'INVALID_INPUT':
      return 'Invalid input'
    case 'NOT_FOUND':
      return 'Not found'
    case 'NOT_CONNECTED':
      return 'Not connected'
    case 'AUTH_FAILED':
      return 'Authentication failed'
    case 'UNREACHABLE':
      return 'Server unreachable'
    case 'TIMEOUT':
      return 'Timed out'
    case 'CANCELED':
      return 'Canceled'
    case 'READ_ONLY':
      return 'Read-only connection'
    case 'CONFLICT':
      return 'Conflict'
    case 'MONGO_ERROR':
      return 'MongoDB error'
    case 'UNSUPPORTED':
      return 'Not supported'
    case 'SECRET_BACKEND':
      return 'Credential store'
    default:
      return 'Error'
  }
}

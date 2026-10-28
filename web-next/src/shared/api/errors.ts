import type { APIError } from './types';

/**
 * definitiveRejection reports whether the center certainly refused a request.
 * Any other failure leaves the outcome unknown: retry the same request.
 */
export function definitiveRejection(error: unknown): boolean {
  return [400, 401, 403, 404, 409, 422].includes((error as APIError)?.status as number);
}

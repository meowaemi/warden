// The center sends Go's zero time (0001-01-01T00:00:00Z) for a site that
// has never had a successful observation.
export function neverObserved(lastSuccess: string): boolean {
  return !lastSuccess || lastSuccess.startsWith('0001-');
}

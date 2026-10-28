/** neverObserved reports a site with no successful snapshot; the center sends Go's zero time for it. */
export function neverObserved(lastSuccess: string): boolean {
  return !lastSuccess || lastSuccess.startsWith('0001-');
}

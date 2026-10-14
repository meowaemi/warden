// Client for the warden backend API.

/**
 * Loads the current problems.
 * @returns {Promise<Array<object>>} messages, oldest first
 */
export async function fetchMessages() {
  const res = await fetch('/api/messages')
  if (!res.ok) {
    throw new Error(`GET /api/messages: ${res.status} ${res.statusText}`)
  }
  return res.json()
}

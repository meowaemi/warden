async function request(path, token, options = {}) {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 15000)
  try {
    const res = await fetch(path, { ...options, signal: controller.signal, headers: { Authorization: `Bearer ${token}`, ...(options.body ? { 'Content-Type': 'application/json' } : {}) } })
    if (!res.ok) {
      const error = new Error(`Request failed (${res.status}). ${res.status === 401 || res.status === 403 ? 'Check the operator token.' : 'Please try again.'}`)
      error.status = res.status
      throw error
    }
    return await res.json()
  } finally { clearTimeout(timeout) }
}
export const fetchSites = token => request('/api/sites', token)
export const fetchCommands = (token, siteID) => request(`/api/commands?site_id=${encodeURIComponent(siteID)}`, token)
export const createCommand = (token, command) => request('/api/commands', token, { method: 'POST', body: JSON.stringify(command) })

export const fetchHistory = (token, siteID) => request(`/api/history?site_id=${encodeURIComponent(siteID)}`, token)

export const fetchDemoLink = token => request('/api/demo/link', token)
export const setDemoLink = (token, connected) => request('/api/demo/link', token, { method: 'POST', body: JSON.stringify({ connected }) })

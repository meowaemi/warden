import test from 'node:test'
import assert from 'node:assert/strict'
import { fetchSites, fetchCommands, fetchHistory, createCommand } from './api.js'

test('authenticated requests use exact API routes and escaped site IDs', async () => {
  const calls = []
  const original = globalThis.fetch
  globalThis.fetch = async (path, options) => {
    calls.push({ path, options })
    return { ok: true, json: async () => [] }
  }
  try {
    await fetchSites('operator-secret')
    await fetchCommands('operator-secret', 'east/one?x=1')
    await fetchHistory('operator-secret', 'east/one?x=1')
    assert.equal(calls[0].path, '/api/sites')
    assert.equal(calls[1].path, '/api/commands?site_id=east%2Fone%3Fx%3D1')
    assert.equal(calls[2].path, '/api/history?site_id=east%2Fone%3Fx%3D1')
    assert.equal(calls[0].options.headers.Authorization, 'Bearer operator-secret')
  } finally { globalThis.fetch = original }
})

test('retrying a command preserves its idempotency key and expiry', async () => {
  const calls = []
  const original = globalThis.fetch
  const command = { id: 'same-request', site_id: 'east', kind: 'restart_deployment', namespace: 'demo', name: 'app', target_uid: 'uid-1', expires_at: '2030-01-01T00:00:00Z' }
  globalThis.fetch = async (_path, options) => {
    calls.push(options)
    if (calls.length === 1) throw new TypeError('Lost response')
    return { ok: true, json: async () => ({ ...command, status: 'queued' }) }
  }
  try {
    await assert.rejects(createCommand('token', command), /Lost response/)
    assert.equal((await createCommand('token', command)).status, 'queued')
    assert.equal(calls[0].body, calls[1].body)
    assert.equal(calls[1].method, 'POST')
  } finally { globalThis.fetch = original }
})

test('HTTP errors expose status for distinguishing rejected from ambiguous requests', async () => {
  const original = globalThis.fetch
  globalThis.fetch = async () => ({ ok: false, status: 409 })
  try { await assert.rejects(createCommand('token', {}), e => e.status === 409) }
  finally { globalThis.fetch = original }
})

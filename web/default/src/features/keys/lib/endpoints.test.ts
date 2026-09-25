/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/

import assert from 'node:assert/strict'
import { test } from 'node:test'
import { measureEndpoint, normalizeApiOrigin, protocolUrl } from './endpoints'

test('endpoint normalization and protocol paths', () => {
  assert.equal(protocolUrl('https://kele520.com/v1/', 'openai'), 'https://kele520.com/v1')
  assert.equal(protocolUrl('https://kele520.com/v1', 'claude'), 'https://kele520.com')
  assert.equal(normalizeApiOrigin('javascript:alert(1)'), 'https://code28.ccwu.cc')
})
test('network probe makes three unauthenticated uncached requests', async () => {
  const original = globalThis.fetch
  const calls: RequestInit[] = []
  globalThis.fetch = (async (_input: unknown, init?: RequestInit) => {
    calls.push(init || {})
    return new Response(null, { status: 204 })
  }) as typeof fetch
  try {
    const result = await measureEndpoint('https://kele520.com')
    assert.equal(result.status, 'success')
    assert.equal(calls.length, 3)
    assert.ok(calls.every(call => call.credentials === 'omit' && call.cache === 'no-store' && !call.headers))
  } finally { globalThis.fetch = original }
})
test('probe treats a returned HTML challenge as a connection failure', async () => {
  const original = globalThis.fetch
  globalThis.fetch = (async () => new Response('challenge', { status: 200 })) as typeof fetch
  try { assert.equal((await measureEndpoint('https://kele520.com')).status, 'error') }
  finally { globalThis.fetch = original }
})

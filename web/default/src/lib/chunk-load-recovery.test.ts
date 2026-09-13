/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// @ts-expect-error -- bun:test is provided by Bun and excluded from app types.
import { describe, expect, test } from 'bun:test'
import { claimChunkReload, isChunkLoadError } from './chunk-load-recovery'

function storageFixture() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
  }
}
describe('chunk failure recovery', () => {
  test('recognizes bundler and browser module failures', () => {
    for (const error of [
      { name: 'ChunkLoadError' },
      { code: 'CSS_CHUNK_LOAD_FAILED' },
      new Error('Loading CSS chunk 42 failed.'),
      new TypeError(
        'Failed to fetch dynamically imported module: /assets/a.js'
      ),
      new TypeError('error loading dynamically imported module'),
      new TypeError('Importing a module script failed.'),
    ])
      expect(isChunkLoadError(error)).toBe(true)
  })
  test('does not reload API errors or application bugs', () => {
    for (const error of [
      null,
      undefined,
      'ChunkLoadError',
      new Error('Failed to fetch'),
      new TypeError('items.map is not a function'),
      { response: { status: 500 } },
    ])
      expect(isChunkLoadError(error)).toBe(false)
  })
  test('limits retries across reloads and different chunks', () => {
    const storage = storageFixture()
    expect(claimChunkReload(storage, 100_000)).toBe(true)
    expect(claimChunkReload(storage, 100_000)).toBe(false)
    expect(claimChunkReload(storage, 159_999)).toBe(false)
    expect(claimChunkReload(storage, 160_000)).toBe(true)
  })
  test('clock changes do not bypass loop protection', () => {
    const storage = storageFixture()
    expect(claimChunkReload(storage, 100_000)).toBe(true)
    expect(claimChunkReload(storage, 90_000)).toBe(false)
  })
  test('blocked or non-persistent storage leaves recovery to manual retry', () => {
    expect(
      claimChunkReload({
        getItem: () => {
          throw Error('blocked')
        },
        setItem: () => {},
      })
    ).toBe(false)
    expect(
      claimChunkReload({
        getItem: () => null,
        setItem: () => {
          throw Error('quota')
        },
      })
    ).toBe(false)
    expect(claimChunkReload({ getItem: () => null, setItem: () => {} })).toBe(
      false
    )
  })
})

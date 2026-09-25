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
export type ApiProtocol = 'openai' | 'claude'
export function normalizeApiOrigin(value: string): string {
  try {
    const url = new URL(value)
    if (url.protocol === 'https:' || url.protocol === 'http:') return url.origin
  } catch {
    /* use installation default */
  }
  return 'https://code28.ccwu.cc'
}
export function protocolUrl(origin: string, protocol: ApiProtocol): string {
  const base = normalizeApiOrigin(origin)
  return protocol === 'openai' ? base + '/v1' : base
}
export type LatencyResult = {
  ms: number | null
  testedAt: number
  status: 'success' | 'timeout' | 'error'
}
export async function measureEndpoint(
  origin: string,
  signal?: AbortSignal
): Promise<LatencyResult> {
  const samples: number[] = []
  try {
    for (let i = 0; i < 3; i++) {
      const controller = new AbortController()
      const abort = () => controller.abort()
      signal?.addEventListener('abort', abort, { once: true })
      if (signal?.aborted) controller.abort()
      let timedOut = false
      const timeout = setTimeout(() => {
        timedOut = true
        controller.abort()
      }, 5000)
      const start = performance.now()
      try {
        const res = await fetch(
          normalizeApiOrigin(origin) +
            '/speedtest/ping?t=' +
            Date.now() +
            '&sample=' +
            i,
          {
            cache: 'no-store',
            credentials: 'omit',
            redirect: 'error',
            signal: controller.signal,
          }
        )
        if (res.status !== 204) throw new Error('Unexpected probe response')
        samples.push(performance.now() - start)
      } catch (err) {
        if (timedOut)
          return { ms: null, testedAt: Date.now(), status: 'timeout' }
        throw err
      } finally {
        clearTimeout(timeout)
        signal?.removeEventListener('abort', abort)
      }
    }
    samples.sort((a, b) => a - b)
    return {
      ms: Math.round(samples[1]),
      testedAt: Date.now(),
      status: 'success',
    }
  } catch {
    return { ms: null, testedAt: Date.now(), status: 'error' }
  }
}

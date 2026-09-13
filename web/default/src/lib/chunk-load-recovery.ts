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
const RELOAD_STORAGE_KEY = 'newapi:chunk-recovery-at'
const RELOAD_COOLDOWN_MS = 60_000
type RecoveryStorage = Pick<Storage, 'getItem' | 'setItem'>
export function isChunkLoadError(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false
  const { name, message, code } = error as Record<string, unknown>
  if (name === 'ChunkLoadError' || code === 'CSS_CHUNK_LOAD_FAILED') return true
  return (
    typeof message === 'string' &&
    /Loading (?:CSS )?chunk [\s\S]* failed|Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed/i.test(
      message
    )
  )
}
// Persist one tab-wide budget across reloads, including failures of different chunks.
export function claimChunkReload(
  storage: RecoveryStorage,
  now = Date.now()
): boolean {
  try {
    const saved = storage.getItem(RELOAD_STORAGE_KEY)
    const previous = saved === null ? NaN : Number(saved)
    if (Number.isFinite(previous) && now - previous < RELOAD_COOLDOWN_MS)
      return false
    storage.setItem(RELOAD_STORAGE_KEY, String(now))
    return storage.getItem(RELOAD_STORAGE_KEY) === String(now)
  } catch {
    // Without durable storage, leave recovery to the explicit retry button.
    return false
  }
}
export function recoverChunkLoad(error: unknown): void {
  if (!isChunkLoadError(error) || typeof window === 'undefined') return
  try {
    if (claimChunkReload(window.sessionStorage)) window.location.reload()
  } catch {
    // Accessing sessionStorage itself can throw when storage is disabled.
  }
}

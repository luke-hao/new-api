/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
export const MANUAL_STABILITY_REFRESH_MS = 5000

type Status = {
  running: boolean
  models?: Array<{ model?: string; running: boolean }>
}

type Options<T extends Status> = {
  load: (signal: AbortSignal) => Promise<T>
  update: (status: T) => void
  error: (error: unknown) => void
  intervalMs?: number
}

// Watching starts only after a manual run has been accepted. Automatic server
// runs never start this timer, and unrelated models never keep it alive.
export function createManualStabilityRefresh<T extends Status>(
  options: Options<T>
) {
  const watched = new Set<string | undefined>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let request: AbortController | undefined
  let version = 0
  let failures = 0

  const schedule = (delay: number) => {
    if (timer !== undefined) clearTimeout(timer)
    timer = setTimeout(() => {
      timer = undefined
      void poll()
    }, delay)
  }

  const poll = async () => {
    if (request || watched.size === 0) return
    const controller = new AbortController()
    const startedVersion = version
    request = controller
    try {
      const status = await options.load(controller.signal)
      if (controller.signal.aborted) return
      // A second manual start may have happened after this GET began. Read a
      // fresh snapshot before deciding that the new task has already completed.
      if (version !== startedVersion) return
      failures = 0
      for (const model of watched) {
        const running =
          model === undefined
            ? status.running
            : status.models?.some((row) => row.model === model && row.running)
        if (!running) watched.delete(model)
      }
      options.update(status) // Includes the final completed ranking.
    } catch (error) {
      if (controller.signal.aborted || version !== startedVersion) return
      failures++
      if (failures >= 3) {
        watched.clear()
        options.error(error)
      }
    } finally {
      if (request === controller) request = undefined
      if (!controller.signal.aborted && watched.size > 0) {
        schedule(
          version === startedVersion
            ? (options.intervalMs ?? MANUAL_STABILITY_REFRESH_MS)
            : 0
        )
      }
    }
  }

  return {
    start(model?: string) {
      watched.add(model)
      version++
      failures = 0
      if (!request) schedule(0)
    },
    stop() {
      version++
      watched.clear()
      if (timer !== undefined) clearTimeout(timer)
      timer = undefined
      request?.abort()
      request = undefined
    },
  }
}

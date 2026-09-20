/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { describe, expect, test } from 'bun:test'
import { createManualStabilityRefresh } from '../src/features/channels/lib/manual-stability-refresh'

const wait = (ms = 30) => new Promise(resolve => setTimeout(resolve, ms))
const status = (running: boolean) => ({ running, models: [{model: 'A',running}] })
describe('manual stability refresh lifecycle', () => {
 test('idle does not read even if the backend may have automatic jobs',async () => {
  let reads=0
  const monitor=createManualStabilityRefresh({load:async()=>{reads++;return status(true)},update:()=>{},error:()=>{},intervalMs:5})
  await wait()
  expect(reads).toBe(0);monitor.stop()
 })
 test('manual model polls until completion and publishes final ranking once',async()=>{
  let reads=0,updates=0
  const monitor=createManualStabilityRefresh({load:async()=>status(++reads<3),update:()=>{updates++},error:()=>{},intervalMs:5})
  monitor.start('A');await wait(60)
  expect(reads).toBe(3);expect(updates).toBe(3)
  await wait();expect(reads).toBe(3);monitor.stop()
 })
 test('an unrelated automatically running model does not prolong a manual model watch',async()=>{
  let reads=0
  const monitor=createManualStabilityRefresh({load:async()=>{reads++;return {running:true,models:[{model:'A',running:false},{model:'B',running:true}]}},update:()=>{},error:()=>{},intervalMs:5})
  monitor.start('A');await wait()
  expect(reads).toBe(1);monitor.stop()
 })
 test('whole group waits until all models finish, including queued ones',async()=>{
  let reads=0
  const monitor=createManualStabilityRefresh({load:async()=>({running:++reads<3,models:[]}),update:()=>{},error:()=>{},intervalMs:5})
  monitor.start();await wait(60);expect(reads).toBe(3);monitor.stop()
 })
 test('fast completion stops after the first status read',async()=>{
  let reads=0
  const monitor=createManualStabilityRefresh({load:async()=>{reads++;return status(false)},update:()=>{},error:()=>{},intervalMs:5})
  monitor.start('A');await wait();expect(reads).toBe(1);monitor.stop()
 })
 test('three read failures stop the temporary watch and report once',async()=>{
  let reads=0,errors=0
  const monitor=createManualStabilityRefresh({load:async()=>{reads++;throw new Error('offline')},update:()=>{},error:()=>{errors++},intervalMs:5})
  monitor.start('A');await wait(60)
  expect(reads).toBe(3);expect(errors).toBe(1);monitor.stop()
 })
 test('unmount aborts the request and discards late updates',async()=>{
  let signal:AbortSignal|undefined,updates=0
  const monitor=createManualStabilityRefresh({load:async(s)=>{signal=s;await wait();return status(true)},update:()=>{updates++},error:()=>{},intervalMs:5})
  monitor.start('A');await wait(5);monitor.stop();await wait(50)
  expect(signal?.aborted).toBe(true);expect(updates).toBe(0)
 })
 test('a new manual run during an older read gets a fresh read, without overlap',async()=>{
  let reads=0,concurrent=0,maxConcurrent=0,updates=0
  const monitor=createManualStabilityRefresh({load:async()=>{reads++;maxConcurrent=Math.max(maxConcurrent,++concurrent);await wait(15);concurrent--;return status(false)},update:()=>{updates++},error:()=>{},intervalMs:5})
  monitor.start('A');await wait(5);monitor.start('B');await wait(65)
  expect(reads).toBe(2);expect(updates).toBe(1);expect(maxConcurrent).toBe(1);monitor.stop()
 })
})

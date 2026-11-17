import { describe, expect, it } from 'vitest'
import { useJudgePolling } from './useJudgePolling'
describe('Judge polling budget', () => {
  it('stops on terminal states and errors, bounds stalled jobs, and can resume', () => {
    let now = 0
    const polling = useJudgePolling(() => now)
    expect(polling.interval(true, null, 2500)).toBe(2500)
    expect(polling.interval(false, null, 2500)).toBe(false)
    expect(polling.interval(true, new Error('offline'), 2500)).toBe(false)
    now = 300_000
    expect(polling.interval(true, null, 2500)).toBe(false)
    expect(polling.paused.value).toBe(true)
    polling.reset()
    expect(polling.paused.value).toBe(false)
    expect(polling.interval(true, null, 2500)).toBe(2500)
  })
})

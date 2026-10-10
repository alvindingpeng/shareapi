import { api } from '@/lib/api'

import type { ProbeCostSummary, ProbeCostSummaryResponse } from './types'

const EMPTY_SUMMARY: ProbeCostSummary = {
  total_probes: 0,
  total_cost: 0,
  start_at: 0,
  end_at: 0,
}

export async function getProbeCostSummary(
  startAt?: number,
  endAt?: number,
): Promise<ProbeCostSummary> {
  const res = await api.get<ProbeCostSummaryResponse>(
    '/api/marketplace/probe-costs',
    { params: { start_at: startAt, end_at: endAt } },
  )
  return res.data.data ?? EMPTY_SUMMARY
}

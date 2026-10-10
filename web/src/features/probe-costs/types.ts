export interface ProbeCostSummary {
  total_probes: number
  total_cost: number
  start_at: number
  end_at: number
}

export interface ProbeCostSummaryResponse {
  success: boolean
  message: string
  data: ProbeCostSummary
}

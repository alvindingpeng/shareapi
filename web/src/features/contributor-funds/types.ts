export interface LedgerBalance {
  matured: number
  pending: number
  total: number
}

export interface LedgerBalanceResponse {
  success: boolean
  message: string
  data: LedgerBalance
}

export type WithdrawalStatus = 'pending' | 'approved' | 'rejected'

export interface Withdrawal {
  id: number
  contributor_id: number
  amount: number
  status: WithdrawalStatus
  note: string
  reviewed_by: number
  created_at: number
  reviewed_at: number
}

export interface WithdrawalsResponse {
  success: boolean
  message: string
  data: Withdrawal[]
}

export interface WithdrawalResponse {
  success: boolean
  message: string
  data: Withdrawal
}

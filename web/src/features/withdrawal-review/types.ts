export interface Withdrawal {
  id: number
  contributor_id: number
  amount: number
  status: 'pending' | 'approved' | 'rejected'
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

export type WithdrawalStatus = Withdrawal['status']

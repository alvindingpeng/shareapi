import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type { Withdrawal, WithdrawalsResponse, WithdrawalStatus } from './types'

const BASE = '/api/withdrawals'

export async function listWithdrawals(status?: WithdrawalStatus): Promise<Withdrawal[]> {
  const response = await api.get<WithdrawalsResponse>(BASE, {
    params: status ? { status } : undefined,
  })
  requireServerSuccess(response.data)
  return response.data.data
}

export async function reviewWithdrawal(
  id: number,
  approve: boolean,
  reason?: string,
): Promise<void> {
  const response = await api.post<{ success: boolean; message: string }>(
    `${BASE}/${id}/review`,
    { approve, reason: reason || undefined },
  )
  requireServerSuccess(response.data)
}

export type { Withdrawal, WithdrawalStatus }

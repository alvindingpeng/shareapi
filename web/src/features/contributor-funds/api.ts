import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  LedgerBalance,
  LedgerBalanceResponse,
  Withdrawal,
  WithdrawalResponse,
  WithdrawalsResponse,
} from './types'

export async function getLedgerBalance(): Promise<LedgerBalance> {
  const response = await api.get<LedgerBalanceResponse>('/api/contributor/ledger/balance')
  requireServerSuccess(response.data)
  return response.data.data
}

export async function requestWithdrawal(amount: number, note: string): Promise<Withdrawal> {
  const response = await api.post<WithdrawalResponse>('/api/contributor/withdrawals/', {
    amount,
    note,
  })
  requireServerSuccess(response.data)
  return response.data.data
}

export async function listWithdrawals(): Promise<Withdrawal[]> {
  const response = await api.get<WithdrawalsResponse>('/api/contributor/withdrawals/')
  requireServerSuccess(response.data)
  return response.data.data
}

export type { LedgerBalance, Withdrawal }

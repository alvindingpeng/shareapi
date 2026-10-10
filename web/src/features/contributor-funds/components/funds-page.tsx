import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useQuery, useQueryClient } from '@tanstack/react-query'

import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'

import { getLedgerBalance, listWithdrawals } from '../api'
import type { Withdrawal, WithdrawalStatus } from '../types'
import { WithdrawDialog } from './withdraw-dialog'

function WithdrawalStatusCell(props: { status: WithdrawalStatus }) {
  const { t } = useTranslation()
  switch (props.status) {
    case 'approved':
      return <StatusBadge variant="success" label={t('Approved')} size="sm" />
    case 'rejected':
      return <StatusBadge variant="danger" label={t('Rejected')} size="sm" />
    default:
      return <StatusBadge variant="warning" label={t('Pending review')} size="sm" />
  }
}

function formatTime(ts: number): string {
  if (!ts) return '—'
  return dayjs(ts * 1000).format('YYYY-MM-DD HH:mm')
}

export function FundsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [withdrawOpen, setWithdrawOpen] = useState(false)

  const balanceQuery = useQuery({
    queryKey: ['contributor-funds-balance'],
    queryFn: getLedgerBalance,
  })

  const withdrawalsQuery = useQuery({
    queryKey: ['contributor-withdrawals'],
    queryFn: listWithdrawals,
  })

  const balance = balanceQuery.data
  const withdrawals: Withdrawal[] = withdrawalsQuery.data ?? []
  const loading = balanceQuery.isLoading || withdrawalsQuery.isLoading

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['contributor-funds-balance'] })
    queryClient.invalidateQueries({ queryKey: ['contributor-withdrawals'] })
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t('My Funds')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Earnings mature 7 days after settlement, then become withdrawable.')}
          </p>
        </div>
        <Button
          onClick={() => setWithdrawOpen(true)}
          disabled={!balance || balance.matured <= 0}
        >
          {t('Withdraw')}
        </Button>
      </div>

      {loading && <LoadingState />}

      {!loading && balance && (
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {t('Available Balance')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatQuotaWithCurrency(balance.matured)}
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t('Matured and withdrawable')}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {t('Pending Maturation')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatQuotaWithCurrency(balance.pending)}
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t('Matures 7 days after settlement')}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {t('Total Balance')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatQuotaWithCurrency(balance.total)}
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                {t('Available + pending')}
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {!loading && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold">{t('Withdrawal History')}</h2>
          {withdrawals.length === 0 ? (
            <EmptyState
              title={t('No withdrawals yet')}
              description={t('Your withdrawal requests will appear here.')}
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Amount')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Note')}</TableHead>
                  <TableHead>{t('Requested at')}</TableHead>
                  <TableHead>{t('Reviewed at')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {withdrawals.map((w) => (
                  <TableRow key={w.id}>
                    <TableCell className="font-medium">
                      {formatQuotaWithCurrency(w.amount)}
                    </TableCell>
                    <TableCell>
                      <WithdrawalStatusCell status={w.status} />
                    </TableCell>
                    <TableCell className="max-w-48 truncate">{w.note || '—'}</TableCell>
                    <TableCell>{formatTime(w.created_at)}</TableCell>
                    <TableCell>{formatTime(w.reviewed_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      )}

      <WithdrawDialog
        open={withdrawOpen}
        onOpenChange={setWithdrawOpen}
        maturedQuota={balance?.matured ?? 0}
        onSubmitted={refresh}
      />
    </div>
  )
}

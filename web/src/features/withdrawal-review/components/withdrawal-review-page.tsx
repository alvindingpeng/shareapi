import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { handleServerError } from '@/lib/handle-server-error'

import { listWithdrawals, reviewWithdrawal } from '../api'
import type { Withdrawal, WithdrawalStatus } from '../types'

const STATUS_VARIANTS: Record<WithdrawalStatus, 'warning' | 'success' | 'danger'> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
}

function StatusCell(props: { status: WithdrawalStatus }) {
  const { t } = useTranslation()
  const labels: Record<WithdrawalStatus, string> = {
    pending: t('Pending'),
    approved: t('Approved'),
    rejected: t('Rejected'),
  }
  return (
    <StatusBadge
      variant={STATUS_VARIANTS[props.status]}
      label={labels[props.status]}
      size="sm"
    />
  )
}

function WithdrawalRow(props: {
  withdrawal: Withdrawal
  onApprove: (w: Withdrawal) => void
  onReject: (w: Withdrawal) => void
}) {
  const { t } = useTranslation()
  const w = props.withdrawal
  const isPending = w.status === 'pending'

  return (
    <TableRow>
      <TableCell className="font-medium">#{w.id}</TableCell>
      <TableCell>{w.contributor_id}</TableCell>
      <TableCell>{formatQuotaWithCurrency(w.amount)}</TableCell>
      <TableCell className="max-w-48 truncate">{w.note || '—'}</TableCell>
      <TableCell>
        <StatusCell status={w.status} />
      </TableCell>
      <TableCell className="whitespace-nowrap">
        {dayjs.unix(w.created_at).format('YYYY-MM-DD HH:mm')}
      </TableCell>
      <TableCell className="text-right">
        {isPending ? (
          <div className="flex justify-end gap-2">
            <Button size="sm" onClick={() => props.onApprove(w)}>
              {t('Approve')}
            </Button>
            <Button size="sm" variant="destructive" onClick={() => props.onReject(w)}>
              {t('Reject')}
            </Button>
          </div>
        ) : (
          <span className="text-sm text-muted-foreground">—</span>
        )}
      </TableCell>
    </TableRow>
  )
}

export function WithdrawalReviewPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<string>('pending')
  const [approveTarget, setApproveTarget] = useState<Withdrawal | null>(null)
  const [rejectTarget, setRejectTarget] = useState<Withdrawal | null>(null)
  const [rejectReason, setRejectReason] = useState('')

  const withdrawalsQuery = useQuery({
    queryKey: ['admin-withdrawals', statusFilter],
    queryFn: () =>
      listWithdrawals(
        statusFilter === 'all' ? undefined : (statusFilter as WithdrawalStatus),
      ),
  })

  const reviewMutation = useMutation({
    mutationFn: ({
      id,
      approve,
      reason,
    }: {
      id: number
      approve: boolean
      reason?: string
    }) => reviewWithdrawal(id, approve, reason),
    onSuccess: () => {
      setApproveTarget(null)
      setRejectTarget(null)
      setRejectReason('')
      queryClient.invalidateQueries({ queryKey: ['admin-withdrawals'] })
    },
    onError: (error) => handleServerError(error, t('Failed to review withdrawal')),
  })

  const withdrawals = withdrawalsQuery.data ?? []

  const handleRejectConfirm = () => {
    if (!rejectTarget) return
    reviewMutation.mutate({
      id: rejectTarget.id,
      approve: false,
      reason: rejectReason.trim() || undefined,
    })
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t('Withdrawal Review')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Review contributor withdrawal requests.')}
          </p>
        </div>
        <Select
          value={statusFilter}
          onValueChange={(value) => setStatusFilter(value ?? 'pending')}
        >
          <SelectTrigger className="w-40">
            <SelectValue placeholder={t('Status')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t('All')}</SelectItem>
            <SelectItem value="pending">{t('Pending')}</SelectItem>
            <SelectItem value="approved">{t('Approved')}</SelectItem>
            <SelectItem value="rejected">{t('Rejected')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {withdrawalsQuery.isLoading && <LoadingState />}
      {!withdrawalsQuery.isLoading && withdrawals.length === 0 && (
        <EmptyState
          title={t('No withdrawal requests')}
          description={t('There are no withdrawal requests with this status.')}
        />
      )}
      {!withdrawalsQuery.isLoading && withdrawals.length > 0 && (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ID')}</TableHead>
                <TableHead>{t('Contributor')}</TableHead>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Note')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Requested at')}</TableHead>
                <TableHead className="text-right">{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {withdrawals.map((w) => (
                <WithdrawalRow
                  key={w.id}
                  withdrawal={w}
                  onApprove={setApproveTarget}
                  onReject={setRejectTarget}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <ConfirmDialog
        open={approveTarget !== null}
        onOpenChange={(open) => {
          if (!open) setApproveTarget(null)
        }}
        title={t('Approve withdrawal')}
        desc={t('Approve this withdrawal request? The amount will be paid out.')}
        confirmText={t('Approve')}
        handleConfirm={() => {
          if (approveTarget) {
            reviewMutation.mutate({ id: approveTarget.id, approve: true })
          }
        }}
        isLoading={reviewMutation.isPending}
      />

      <ConfirmDialog
        open={rejectTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRejectTarget(null)
            setRejectReason('')
          }
        }}
        title={t('Reject withdrawal')}
        desc={t('Reject this withdrawal request? The held amount will be returned.')}
        confirmText={t('Reject')}
        destructive
        handleConfirm={handleRejectConfirm}
        isLoading={reviewMutation.isPending}
      >
        <div className="space-y-2">
          <label
            htmlFor="reject-reason"
            className="text-sm font-medium text-muted-foreground"
          >
            {t('Reject reason (optional)')}
          </label>
          <Textarea
            id="reject-reason"
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder={t('Enter the reason for rejection')}
            rows={3}
          />
        </div>
      </ConfirmDialog>
    </div>
  )
}

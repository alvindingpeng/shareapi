import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatQuotaWithCurrency, getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { handleServerError } from '@/lib/handle-server-error'

import { requestWithdrawal } from '../api'

interface WithdrawDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  maturedQuota: number
  onSubmitted: () => void
}

export function WithdrawDialog(props: WithdrawDialogProps) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const { config } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()

  const maxUsd = props.maturedQuota / config.quotaPerUnit

  const withdrawSchema = z.object({
    amount: z
      .number({ invalid_type_error: t('Amount must be a number') })
      .positive(t('Amount must be greater than 0'))
      .max(maxUsd, t('Amount cannot exceed available balance')),
    note: z.string().max(500).optional().default(''),
  })

  type WithdrawForm = z.infer<typeof withdrawSchema>

  const form = useForm<WithdrawForm>({
    resolver: zodResolver(withdrawSchema),
    defaultValues: { amount: 0, note: '' },
  })

  const onSubmit = async (values: WithdrawForm) => {
    setSubmitting(true)
    try {
      const quotaAmount = Math.floor(values.amount * config.quotaPerUnit)
      await requestWithdrawal(quotaAmount, values.note?.trim() ?? '')
      form.reset()
      props.onOpenChange(false)
      props.onSubmitted()
    } catch (error) {
      handleServerError(error, t('Failed to submit withdrawal request'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Request withdrawal')}
      description={t('Withdrawals are reviewed manually. Approved payouts are deducted from your available balance.')}
    >
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="wd-amount">
            {t('Amount')} ({currencyLabel})
          </Label>
          <Input
            id="wd-amount"
            type="number"
            step="0.01"
            min="0"
            placeholder="0.00"
            {...form.register('amount', { valueAsNumber: true })}
          />
          <p className="text-xs text-muted-foreground">
            {t('Available')}: {formatQuotaWithCurrency(props.maturedQuota)}
          </p>
          {form.formState.errors.amount && (
            <p className="text-sm text-destructive">{form.formState.errors.amount.message}</p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="wd-note">{t('Note (optional)')}</Label>
          <Textarea
            id="wd-note"
            rows={3}
            placeholder={t('Payment account or remark for the reviewer')}
            {...form.register('note')}
          />
        </div>

        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => props.onOpenChange(false)}
            disabled={submitting}
          >
            {t('Cancel')}
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting ? t('Submitting...') : t('Submit')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useQuery } from '@tanstack/react-query'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { formatNumber } from '@/lib/format'

import { getProbeCostSummary } from '../api'

type RangeDays = 7 | 30 | 90

function StatCard(props: { label: string; value: string }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {props.label}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-2xl font-semibold">{props.value}</p>
      </CardContent>
    </Card>
  )
}

export function ProbeCostsPage() {
  const { t, i18n } = useTranslation()
  const [range, setRange] = useState<RangeDays>(30)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const summaryQuery = useQuery({
    queryKey: ['probe-costs', range],
    queryFn: () => {
      const endAt = Math.floor(Date.now() / 1000)
      const startAt = endAt - range * 86400
      return getProbeCostSummary(startAt, endAt)
    },
  })

  const summary = summaryQuery.data
  const avgCostPerProbe =
    summary && summary.total_probes > 0
      ? summary.total_cost / summary.total_probes
      : 0

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">{t('Probe Costs')}</h1>
          <p className="text-sm text-muted-foreground">
            {t(
              'Fingerprint probe costs for model verification. All probe costs are borne by the platform.',
            )}
          </p>
        </div>
        <select
          value={range}
          onChange={(e) => setRange(Number(e.target.value) as RangeDays)}
          className="rounded-md border px-3 py-1.5 text-sm"
          aria-label={t('Date range')}
        >
          <option value={7}>{t('Last 7 days')}</option>
          <option value={30}>{t('Last 30 days')}</option>
          <option value={90}>{t('Last 90 days')}</option>
        </select>
      </div>

      {summaryQuery.isLoading && <LoadingState />}
      {summaryQuery.isError && (
        <ErrorState
          title={t('Failed to load probe costs')}
          description={t('Please try again later.')}
          onRetry={() => summaryQuery.refetch()}
        />
      )}
      {!summaryQuery.isLoading && !summaryQuery.isError && summary && (
        <>
          {summary.start_at > 0 && summary.end_at > 0 && (
            <p className="text-sm text-muted-foreground">
              {dayjs.unix(summary.start_at).format('YYYY-MM-DD')}
              {' — '}
              {dayjs.unix(summary.end_at).format('YYYY-MM-DD')}
            </p>
          )}
          {summary.total_probes === 0 ? (
            <EmptyState
              title={t('No probe records')}
              description={t('No fingerprint probes ran in this period.')}
            />
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <StatCard
                label={t('Total Probes')}
                value={formatNumber(summary.total_probes, locale)}
              />
              <StatCard
                label={t('Total Cost')}
                value={formatQuotaWithCurrency(summary.total_cost)}
              />
              <StatCard
                label={t('Average Cost per Probe')}
                value={formatQuotaWithCurrency(avgCostPerProbe)}
              />
            </div>
          )}
        </>
      )}
    </div>
  )
}

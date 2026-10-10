import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
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
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency, getCurrencyDisplay } from '@/lib/currency'
import { formatNumber } from '@/lib/format'

import { getModelChannels, listMarketplaceModels } from '../api'
import type { MarketplaceModelChannel } from '../types'

type SortKey = 'price' | 'trust'

function VerificationBadge(props: { status: number }) {
  const { t } = useTranslation()
  const { status } = props
  if (status === 1) {
    return <StatusBadge variant="success" label={t('Verified')} size="sm" />
  }
  if (status === 2) {
    return <StatusBadge variant="warning" label={t('Suspicious')} size="sm" />
  }
  return <StatusBadge variant="neutral" label={t('Unverified')} size="sm" />
}

function TrustBar(props: { score: number }) {
  const { score } = props
  const pct = Math.max(0, Math.min(100, Math.round(score)))
  let color = 'bg-zinc-300'
  if (pct >= 80) {
    color = 'bg-emerald-500'
  } else if (pct >= 50) {
    color = 'bg-amber-500'
  }
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-700">
        <div className={`h-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs text-muted-foreground">{pct}</span>
    </div>
  )
}

export function ModelDetailPage(props: { modelName: string }) {
  const { t, i18n } = useTranslation()
  const { modelName } = props
  const [sort, setSort] = useState<SortKey>('price')
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const channelsQuery = useQuery({
    queryKey: ['marketplace-model-channels', modelName, sort],
    queryFn: () => getModelChannels(modelName, sort),
  })

  // Reuse the cached model list (same queryKey shape as the marketplace page)
  // to get the platform base price for per-channel price computation.
  const modelsQuery = useQuery({
    queryKey: ['marketplace-models', 'trust'],
    queryFn: () => listMarketplaceModels('trust'),
  })

  const channels = channelsQuery.data ?? []
  const basePrice =
    modelsQuery.data?.find((m) => m.model === modelName)?.base_price ?? 0
  const { config } = getCurrencyDisplay()

  function channelPrice(ch: MarketplaceModelChannel): string {
    // P11: prefer absolute user price; fall back to multiplier × base for legacy data.
    const userPrice = ch.user_price_usd_per_1m ?? 0
    if (userPrice > 0) {
      return formatQuotaWithCurrency(userPrice * config.quotaPerUnit)
    }
    if (basePrice <= 0) return '—'
    const usdPer1M = basePrice * ch.price_multiplier
    return formatQuotaWithCurrency(usdPer1M * config.quotaPerUnit)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <Button render={<Link to="/marketplace" />}>
        {t('Back to marketplace')}
      </Button>

      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold break-all">{modelName}</h1>
          <p className="text-sm text-muted-foreground">
            {t(
              'Compare channels serving this model. Prices are set by each contributor.',
            )}
          </p>
        </div>
        <select
          value={sort}
          onChange={(e) => setSort(e.target.value as SortKey)}
          className="rounded-md border px-3 py-1.5 text-sm"
          aria-label={t('Sort by')}
        >
          <option value="price">{t('Sort by price')}</option>
          <option value="trust">{t('Sort by trust')}</option>
        </select>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>
            {t('Channel comparison')} ({formatNumber(channels.length, locale)})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {channelsQuery.isLoading && <LoadingState />}
          {channelsQuery.isError && (
            <ErrorState
              title={t('Failed to load channels')}
              onRetry={() => channelsQuery.refetch()}
            />
          )}
          {!channelsQuery.isLoading &&
            !channelsQuery.isError &&
            channels.length === 0 && (
              <EmptyState
                title={t('No channels serve this model')}
                description={t(
                  'Contributors have not shared this model yet.',
                )}
              />
            )}
          {!channelsQuery.isLoading &&
            !channelsQuery.isError &&
            channels.length > 0 && (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead>{t('Sharer')}</TableHead>
                      <TableHead>{t('Price per 1M tokens')}</TableHead>
                      <TableHead>{t('Multiplier')}</TableHead>
                      <TableHead>{t('Trust score')}</TableHead>
                      <TableHead>{t('Verification')}</TableHead>
                      <TableHead>{t('Official')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {channels.map((ch) => (
                      <TableRow key={ch.channel_id}>
                        <TableCell className="font-medium">
                          <Link
                            to="/marketplace/channels/$channelId"
                            params={{ channelId: String(ch.channel_id) }}
                            className="text-primary hover:underline"
                          >
                            #{ch.channel_id} {ch.name}
                          </Link>
                        </TableCell>
                        <TableCell>{ch.sharer}</TableCell>
                        <TableCell className="font-medium">
                          {channelPrice(ch)}
                        </TableCell>
                        <TableCell>
                          ×{formatNumber(ch.price_multiplier, locale)}
                        </TableCell>
                        <TableCell>
                          <TrustBar score={ch.trust_score} />
                        </TableCell>
                        <TableCell>
                          <VerificationBadge status={ch.verification_status} />
                        </TableCell>
                        <TableCell>
                          {ch.endpoint_official ? (
                            <StatusBadge
                              variant="success"
                              label={t('Official')}
                              size="sm"
                            />
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
        </CardContent>
      </Card>
    </div>
  )
}

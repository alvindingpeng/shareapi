import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/stores/auth-store'

import { listMarketplaceModels } from '../api'
import type { MarketplaceModel } from '../types'

type SortKey = 'trust' | 'price' | 'channels'

function TrustBadge(props: { score: number; verified: number; total: number }) {
  const { t } = useTranslation()
  const { score, verified, total } = props
  // score is 0-100 (P7-6)
  if (total === 0 || score < 50) {
    return <StatusBadge variant="neutral" label={t('Unverified')} size="sm" />
  }
  if (score >= 80) {
    return (
      <StatusBadge
        variant="success"
        label={`${t('Verified')} ${verified}/${total}`}
        size="sm"
      />
    )
  }
  return (
    <StatusBadge
      variant="warning"
      label={`${t('Partially verified')} ${verified}/${total}`}
      size="sm"
    />
  )
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

function PriceTag(props: { model: MarketplaceModel }) {
  const { t } = useTranslation()
  const { model } = props
  const discount =
    model.base_price > 0 && model.price < model.base_price
      ? Math.round((1 - model.price / model.base_price) * 100)
      : 0
  return (
    <div className="flex items-baseline gap-2">
      <span className="text-lg font-semibold">
        {model.price > 0 ? `$${model.price.toFixed(4)}` : '—'}
      </span>
      <span className="text-xs text-muted-foreground">{t('/ 1M tokens')}</span>
      {discount > 0 && (
        <StatusBadge variant="success" label={`-${discount}%`} size="sm" />
      )}
    </div>
  )
}

function ModelCard(props: { model: MarketplaceModel }) {
  const { t } = useTranslation()
  const { model } = props
  return (
    <Card className="flex flex-col">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <CardTitle className="text-base break-all">{model.model}</CardTitle>
          <TrustBadge
            score={model.trust_score}
            verified={model.verified_channels}
            total={model.channels}
          />
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col justify-between gap-4">
        <PriceTag model={model} />
        <div className="flex items-center justify-between text-sm">
          <TrustBar score={model.trust_score} />
          <span className="text-xs text-muted-foreground">
            {t('{{count}} channels', { count: model.channels })}
          </span>
        </div>
      </CardContent>
    </Card>
  )
}

export function MarketplacePage() {
  const { t } = useTranslation()
  const [sort, setSort] = useState<SortKey>('trust')
  const [query, setQuery] = useState('')
  const { auth } = useAuthStore()

  const modelsQuery = useQuery({
    queryKey: ['marketplace-models', sort],
    queryFn: () => listMarketplaceModels(sort),
  })

  const models = useMemo(() => {
    const all = modelsQuery.data ?? []
    const q = query.trim().toLowerCase()
    if (!q) return all
    return all.filter((m) => m.model.toLowerCase().includes(q))
  }, [modelsQuery.data, query])

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">{t('Model Marketplace')}</h1>
          <p className="text-sm text-muted-foreground">
            {t(
              'Browse models shared by contributors. Prices are set by contributors; trust scores reflect automated verification.',
            )}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t('Search models')}
            className="w-48"
            aria-label={t('Search models')}
          />
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortKey)}
            className="rounded-md border px-3 py-1.5 text-sm"
            aria-label={t('Sort by')}
          >
            <option value="trust">{t('Sort by trust')}</option>
            <option value="price">{t('Sort by price')}</option>
            <option value="channels">{t('Sort by channels')}</option>
          </select>
          {auth ? (
            <Button render={<Link to="/keys" />}>{t('Get API Key')}</Button>
          ) : (
            <Button render={<Link to="/register" />}>
              {t('Sign up to get API key')}
            </Button>
          )}
        </div>
      </div>

      {modelsQuery.isLoading && <LoadingState />}
      {!modelsQuery.isLoading && models.length === 0 && (
        <EmptyState
          title={
            query ? t('No models match your search') : t('No models listed yet')
          }
          description={
            query
              ? t('Try a different search term.')
              : t('Contributors have not shared any channels yet.')
          }
        />
      )}
      {!modelsQuery.isLoading && models.length > 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {models.map((m) => (
            <ModelCard key={m.model} model={m} />
          ))}
        </div>
      )}
    </div>
  )
}

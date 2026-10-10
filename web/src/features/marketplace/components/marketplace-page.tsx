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

import { listMarketplaceChannels, listMarketplaceModels, getMarketplaceStats } from '../api'
import type { MarketplaceChannel, MarketplaceModel } from '../types'

type SortKey = 'trust' | 'price' | 'channels'
type ViewTab = 'models' | 'channels'

function TrustBadge(props: { score: number; verified: number; total: number }) {
  const { t } = useTranslation()
  const { score, verified, total } = props
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

const USD_TO_CNY = 7.2

function formatPrice(usd: number, currency: 'USD' | 'CNY'): string {
  if (usd <= 0) return '—'
  if (currency === 'CNY') {
    return `¥${(usd * USD_TO_CNY).toFixed(4)}`
  }
  return `$${usd.toFixed(4)}`
}

function PriceTag(props: { model: MarketplaceModel; currency: 'USD' | 'CNY' }) {
  const { t } = useTranslation()
  const { model, currency } = props
  const discount =
    model.base_price > 0 && model.price < model.base_price
      ? Math.round((1 - model.price / model.base_price) * 100)
      : 0
  return (
    <div className="flex items-baseline gap-2">
      <span className="text-lg font-semibold">
        {formatPrice(model.price, currency)}
      </span>
      <span className="text-xs text-muted-foreground">{t('/ 1M tokens')}</span>
      {discount > 0 && (
        <StatusBadge variant="success" label={`-${discount}%`} size="sm" />
      )}
    </div>
  )
}

function ModelCard(props: { model: MarketplaceModel; currency: 'USD' | 'CNY' }) {
  const { t } = useTranslation()
  const { model, currency } = props
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
        <PriceTag model={model} currency={currency} />
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

function ChannelCard(props: { channel: MarketplaceChannel }) {
  const { t } = useTranslation()
  const { channel } = props
  let priceLabel = t('Base price')
  if (channel.price_multiplier < 1) {
    priceLabel = `-${Math.round((1 - channel.price_multiplier) * 100)}%`
  } else if (channel.price_multiplier > 1) {
    priceLabel = `+${Math.round((channel.price_multiplier - 1) * 100)}%`
  }
  return (
    <Link
      to="/marketplace/channels/$channelId"
      params={{ channelId: String(channel.channel_id) }}
      className="block"
    >
      <Card className="flex h-full flex-col transition-shadow hover:shadow-md">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div>
            <CardTitle className="text-base break-all">
              #{channel.channel_id} {channel.name}
            </CardTitle>
            <p className="mt-1 text-xs text-muted-foreground">
              {t('Sharer')}: {channel.sharer}
            </p>
          </div>
          <div className="flex flex-col items-end gap-1">
            {channel.endpoint_official ? (
              <StatusBadge variant="success" label={t('Official')} size="sm" />
            ) : (
              <StatusBadge variant="neutral" label={t('Unverified')} size="sm" />
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col justify-between gap-3">
        <div className="flex flex-wrap gap-1">
          {channel.models.slice(0, 4).map((m) => (
            <span
              key={m}
              className="rounded bg-zinc-100 px-1.5 py-0.5 text-xs dark:bg-zinc-800"
            >
              {m}
            </span>
          ))}
          {channel.models.length > 4 && (
            <span className="text-xs text-muted-foreground">
              +{channel.models.length - 4}
            </span>
          )}
        </div>
        <div className="flex items-center justify-between text-sm">
          <TrustBar score={channel.trust_score} />
          <StatusBadge
            variant={channel.price_multiplier <= 1 ? 'success' : 'warning'}
            label={priceLabel}
            size="sm"
          />
        </div>
      </CardContent>
      </Card>
    </Link>
  )
}

export function MarketplacePage() {
  const { t } = useTranslation()
  const [tab, setTab] = useState<ViewTab>('models')
  const [sort, setSort] = useState<SortKey>('trust')
  const [query, setQuery] = useState('')
  const [brand, setBrand] = useState('')
  const [freeOnly, setFreeOnly] = useState(false)
  const [currency, setCurrency] = useState<'USD' | 'CNY'>('USD')
  const { auth } = useAuthStore()

  const modelsQuery = useQuery({
    queryKey: ['marketplace-models', sort],
    queryFn: () => listMarketplaceModels(sort),
    enabled: tab === 'models',
  })

  const channelsQuery = useQuery({
    queryKey: [
      'marketplace-channels',
      sort === 'channels' ? 'trust' : sort,
      query,
      brand,
      freeOnly,
    ],
    queryFn: () =>
      listMarketplaceChannels(
        sort === 'channels' ? 'trust' : sort,
        query,
        brand,
        freeOnly,
      ),
    enabled: tab === 'channels',
  })

  const models = useMemo(() => {
    const all = modelsQuery.data ?? []
    const q = query.trim().toLowerCase()
    if (!q) return all
    return all.filter((m) => m.model.toLowerCase().includes(q))
  }, [modelsQuery.data, query])

  const channels = channelsQuery.data ?? []
  const isLoading =
    tab === 'models' ? modelsQuery.isLoading : channelsQuery.isLoading
  const items = tab === 'models' ? models : channels

  const statsQuery = useQuery({
    queryKey: ['marketplace-stats'],
    queryFn: getMarketplaceStats,
    refetchInterval: 60000, // refresh every minute
  })
  const stats = statsQuery.data

  function formatCompact(n: number): string {
    if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
    if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
    if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
    return `${n}`
  }

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
          {stats && (
            <div className="mt-2 flex gap-4 text-sm text-muted-foreground">
              <span>
                {t('Today calls')}: <strong>{formatCompact(stats.today_calls)}</strong>
              </span>
              <span>
                {t('Today tokens')}: <strong>{formatCompact(stats.today_tokens)}</strong>
              </span>
              <span>
                {t('Shared models')}: <strong>{stats.shared_models}</strong>
              </span>
            </div>
          )}
        </div>
        <div className="flex items-center gap-3">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={
              tab === 'models' ? t('Search models') : t('Search channels or sharers')
            }
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
            {tab === 'models' && (
              <option value="channels">{t('Sort by channels')}</option>
            )}
          </select>
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value as 'USD' | 'CNY')}
            className="rounded-md border px-3 py-1.5 text-sm"
            aria-label={t('Currency')}
          >
            <option value="USD">USD</option>
            <option value="CNY">CNY</option>
          </select>
          {tab === 'channels' && (
            <>
              <select
                value={brand}
                onChange={(e) => setBrand(e.target.value)}
                className="rounded-md border px-3 py-1.5 text-sm"
                aria-label={t('Brand')}
              >
                <option value="">{t('All brands')}</option>
                <option value="1">OpenAI</option>
                <option value="14">Anthropic</option>
                <option value="24">Gemini</option>
                <option value="43">DeepSeek</option>
                <option value="48">xAI</option>
              </select>
              <label className="flex items-center gap-1.5 text-sm">
                <input
                  type="checkbox"
                  checked={freeOnly}
                  onChange={(e) => setFreeOnly(e.target.checked)}
                  className="rounded"
                />
                {t('Free only')}
              </label>
            </>
          )}
          {auth ? (
            <Button render={<Link to="/keys" />}>{t('Get API Key')}</Button>
          ) : (
            <Button render={<Link to="/register" />}>
              {t('Sign up to get API key')}
            </Button>
          )}
        </div>
      </div>

      <div className="flex gap-2 border-b">
        <button
          type="button"
          onClick={() => setTab('models')}
          className={`px-4 py-2 text-sm font-medium ${
            tab === 'models'
              ? 'border-b-2 border-primary text-primary'
              : 'text-muted-foreground'
          }`}
        >
          {t('Models')}
        </button>
        <button
          type="button"
          onClick={() => setTab('channels')}
          className={`px-4 py-2 text-sm font-medium ${
            tab === 'channels'
              ? 'border-b-2 border-primary text-primary'
              : 'text-muted-foreground'
          }`}
        >
          {t('Channels')}
        </button>
      </div>

      {isLoading && <LoadingState />}
      {!isLoading && items.length === 0 && (
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
      {!isLoading && items.length > 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {tab === 'models'
            ? models.map((m) => (
                <ModelCard key={m.model} model={m} currency={currency} />
              ))
            : channels.map((ch) => (
                <ChannelCard key={ch.channel_id} channel={ch} />
              ))}
        </div>
      )}
    </div>
  )
}

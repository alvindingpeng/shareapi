import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useAuthStore } from '@/stores/auth-store'

import { listMarketplaceModels } from '../api'

type SortKey = 'trust' | 'price' | 'channels'

function TrustBadge(props: { score: number; verified: number; total: number }) {
  const { t } = useTranslation()
  const { score, verified, total } = props
  if (total === 0) {
    return <StatusBadge variant="neutral" label={t('Unverified')} size="sm" />
  }
  if (score >= 0.8) {
    return (
      <StatusBadge
        variant="success"
        label={`${t('Verified')} ${verified}/${total}`}
        size="sm"
      />
    )
  }
  if (score >= 0.5) {
    return (
      <StatusBadge
        variant="warning"
        label={`${t('Partially verified')} ${verified}/${total}`}
        size="sm"
      />
    )
  }
  return (
    <StatusBadge
      variant="neutral"
      label={`${t('Unverified')} ${verified}/${total}`}
      size="sm"
    />
  )
}

export function MarketplacePage() {
  const { t } = useTranslation()
  const [sort, setSort] = useState<SortKey>('trust')
  const { auth } = useAuthStore()

  const modelsQuery = useQuery({
    queryKey: ['marketplace-models', sort],
    queryFn: () => listMarketplaceModels(sort),
  })

  const models = modelsQuery.data ?? []

  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t('Model Marketplace')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Browse models shared by contributors. Trust badges reflect automated fingerprint verification.')}
          </p>
        </div>
        <div className="flex items-center gap-3">
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
          title={t('No models listed yet')}
          description={t('Contributors have not shared any channels yet.')}
        />
      )}
      {!modelsQuery.isLoading && models.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Price')}</TableHead>
              <TableHead>{t('Channels')}</TableHead>
              <TableHead>{t('Trust')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {models.map((m) => (
              <TableRow key={m.model}>
                <TableCell className="font-medium">{m.model}</TableCell>
                <TableCell>
                  {m.price > 0 ? `$${m.price.toFixed(4)}` : '—'}
                </TableCell>
                <TableCell>{m.channels}</TableCell>
                <TableCell>
                  <TrustBadge
                    score={m.trust_score}
                    verified={m.verified_channels}
                    total={m.channels}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}

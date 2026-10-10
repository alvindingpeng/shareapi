import { useTranslation } from 'react-i18next'

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { getChannelDetail } from '../api'

function VerdictBadge(props: { verdict: string }) {
  const { t } = useTranslation()
  const { verdict } = props
  if (verdict === 'verified') {
    return <StatusBadge variant="success" label={t('Verified')} size="sm" />
  }
  if (verdict === 'mismatch') {
    return <StatusBadge variant="danger" label={t('Mismatch')} size="sm" />
  }
  return <StatusBadge variant="neutral" label={t('Error')} size="sm" />
}

export function ChannelDetailPage(props: { channelId: number }) {
  const { t } = useTranslation()
  const { channelId } = props

  const query = useQuery({
    queryKey: ['marketplace-channel', channelId],
    queryFn: () => getChannelDetail(channelId),
  })

  if (query.isLoading) return <LoadingState />
  if (!query.data) {
    return (
      <div className="mx-auto max-w-4xl p-6">
        <EmptyState
          title={t('Channel not found')}
          description={t('This channel is not available.')}
        />
        <div className="mt-4">
          <Button render={<Link to="/marketplace" />}>
            {t('Back to marketplace')}
          </Button>
        </div>
      </div>
    )
  }

  const ch = query.data
  const successRate =
    ch.total_probes > 0
      ? Math.round((ch.verified_count / ch.total_probes) * 100)
      : 0

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <Button render={<Link to="/marketplace" />}>{t('Back to marketplace')}</Button>

      <Card>
        <CardHeader>
          <CardTitle>
            #{ch.channel_id} {ch.name}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-4 text-sm md:grid-cols-4">
            <div>
              <p className="text-muted-foreground">{t('Trust score')}</p>
              <p className="text-lg font-semibold">{ch.trust_score}</p>
            </div>
            <div>
              <p className="text-muted-foreground">{t('Price')}</p>
              <p className="text-lg font-semibold">
                {ch.price_multiplier === 0
                  ? t('Free')
                  : `${ch.price_multiplier}x`}
              </p>
            </div>
            <div>
              <p className="text-muted-foreground">{t('Probes')}</p>
              <p className="text-lg font-semibold">{ch.total_probes}</p>
            </div>
            <div>
              <p className="text-muted-foreground">{t('Success rate')}</p>
              <p className="text-lg font-semibold">{successRate}%</p>
            </div>
          </div>
          <div>
            <p className="mb-2 text-sm text-muted-foreground">{t('Models')}</p>
            <div className="flex flex-wrap gap-1">
              {ch.models.map((m) => (
                <span
                  key={m}
                  className="rounded bg-zinc-100 px-2 py-1 text-xs dark:bg-zinc-800"
                >
                  {m}
                </span>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Verification history')}</CardTitle>
        </CardHeader>
        <CardContent>
          {ch.history.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t('No verification records yet.')}
            </p>
          ) : (
            <div className="space-y-2">
              {ch.history.map((h) => (
                <div
                  key={h.id}
                  className="flex items-center justify-between rounded border p-3 text-sm"
                >
                  <div className="flex items-center gap-3">
                    <VerdictBadge verdict={h.verdict} />
                    <span className="text-muted-foreground">
                      {new Date(h.created_at * 1000).toLocaleString()}
                    </span>
                    {h.endpoint_official && (
                      <StatusBadge
                        variant="success"
                        label={t('Official')}
                        size="sm"
                      />
                    )}
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {h.latency_ms}ms
                  </span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

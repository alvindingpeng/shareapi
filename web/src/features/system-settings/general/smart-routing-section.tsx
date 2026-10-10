import { useTranslation } from 'react-i18next'

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'

type SmartRoutingConfig = {
  enabled: boolean
  strategy: 'cheapest' | 'trust_first' | 'balanced'
}

async function fetchSmartRouting(): Promise<SmartRoutingConfig> {
  const res = await api.get('/api/option/smart_routing')
  return res.data.data
}

async function updateSmartRouting(
  config: Partial<SmartRoutingConfig>,
): Promise<SmartRoutingConfig> {
  const res = await api.patch('/api/option/smart_routing', config)
  return res.data.data
}

const STRATEGIES = [
  { value: 'balanced', labelKey: 'Balanced (price + trust)' },
  { value: 'cheapest', labelKey: 'Cheapest price' },
  { value: 'trust_first', labelKey: 'Highest trust' },
] as const

export function SmartRoutingSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const query = useQuery({
    queryKey: ['smart-routing-setting'],
    queryFn: fetchSmartRouting,
  })

  const mutation = useMutation({
    mutationFn: updateSmartRouting,
    onSuccess: (data) => {
      queryClient.setQueryData(['smart-routing-setting'], data)
    },
  })

  const config = query.data

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Smart Routing')}</CardTitle>
        <p className="text-sm text-muted-foreground">
          {t(
            'Automatically select the best contributor channel per request based on price and trust score.',
          )}
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between">
          <Label htmlFor="smart-routing-enabled">{t('Enabled')}</Label>
          <Switch
            id="smart-routing-enabled"
            checked={config?.enabled ?? true}
            disabled={query.isLoading || mutation.isPending}
            onCheckedChange={(checked) => mutation.mutate({ enabled: checked })}
          />
        </div>
        <div className="space-y-2">
          <Label>{t('Strategy')}</Label>
          <select
            value={config?.strategy ?? 'balanced'}
            disabled={query.isLoading || mutation.isPending || !config?.enabled}
            onChange={(e) =>
              mutation.mutate({
                strategy: e.target.value as SmartRoutingConfig['strategy'],
              })
            }
            className="w-full rounded-md border px-3 py-2 text-sm"
            aria-label={t('Routing strategy')}
          >
            {STRATEGIES.map((s) => (
              <option key={s.value} value={s.value}>
                {t(s.labelKey)}
              </option>
            ))}
          </select>
          <p className="text-xs text-muted-foreground">
            {t(
              'Cheapest picks the lowest contributor price. Highest trust picks the best verified channel. Balanced weights both.',
            )}
          </p>
        </div>
      </CardContent>
    </Card>
  )
}

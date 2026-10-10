import { api } from '@/lib/api'

import type {
  MarketplaceChannel,
  MarketplaceChannelsResponse,
  MarketplaceModel,
  MarketplaceResponse,
} from './types'

export async function listMarketplaceModels(
  sort: 'trust' | 'price' | 'channels' = 'trust',
): Promise<MarketplaceModel[]> {
  const res = await api.get<MarketplaceResponse>('/api/marketplace/models', {
    params: { sort },
  })
  return res.data.data ?? []
}

export async function listMarketplaceChannels(
  sort: 'trust' | 'price' = 'trust',
  q = '',
  brand = '',
  freeOnly = false,
): Promise<MarketplaceChannel[]> {
  const res = await api.get<MarketplaceChannelsResponse>(
    '/api/marketplace/channels',
    { params: { sort, q, brand, free_only: freeOnly } },
  )
  return res.data.data ?? []
}

export interface MarketplaceStats {
  today_calls: number
  today_tokens: number
  shared_models: number
}

export async function getMarketplaceStats(): Promise<MarketplaceStats> {
  const res = await api.get('/api/marketplace/stats')
  return (
    res.data.data ?? { today_calls: 0, today_tokens: 0, shared_models: 0 }
  )
}

export interface ChannelDetail {
  channel_id: number
  name: string
  type: number
  models: string[]
  price_multiplier: number
  trust_score: number
  verification_status: number
  last_verified_at: number
  total_probes: number
  verified_count: number
  history: Array<{
    id: number
    verdict: string
    detail: string
    latency_ms: number
    created_at: number
    endpoint_official: boolean
    identity_consistent: boolean
  }>
}

export async function getChannelDetail(id: number): Promise<ChannelDetail | null> {
  const res = await api.get(`/api/marketplace/channels/${id}`)
  if (!res.data.success) return null
  return res.data.data
}

import { api } from '@/lib/api'

import type { MarketplaceModel, MarketplaceResponse } from './types'

export async function listMarketplaceModels(
  sort: 'trust' | 'price' | 'channels' = 'trust',
): Promise<MarketplaceModel[]> {
  const res = await api.get<MarketplaceResponse>('/api/marketplace/models', {
    params: { sort },
  })
  return res.data.data ?? []
}

export interface MarketplaceModel {
  model: string
  channels: number
  verified_channels: number
  trust_score: number // 0-100 average trust score (P7-6)
  price: number // best (lowest) contributor price
  base_price: number // platform base price
  min_multiplier: number // lowest price multiplier
  available: boolean
}

export interface MarketplaceChannel {
  channel_id: number
  name: string
  type: number
  models: string[]
  sharer: string
  price_multiplier: number
  trust_score: number
  verification_status: number
  endpoint_official: boolean
}

export interface MarketplaceResponse {
  success: boolean
  message: string
  data: MarketplaceModel[]
}

export interface MarketplaceChannelsResponse {
  success: boolean
  message: string
  data: MarketplaceChannel[]
}

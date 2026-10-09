export interface MarketplaceModel {
  model: string
  channels: number
  verified_channels: number
  trust_score: number
  price: number
  available: boolean
}

export interface MarketplaceResponse {
  success: boolean
  message: string
  data: MarketplaceModel[]
}

export interface ContributorChannel {
  id: number
  type: number
  name: string
  base_url: string | null
  models: string
  group: string
  status: number
  owner_user_id: number
  created_time: number
  test_time: number
  response_time: number
  balance: number
  used_quota: number
  verification_status: number
  last_verified_at: number
}

export interface ContributorChannelInput {
  type: number
  key: string
  name: string
  base_url?: string | null
  models: string
  price_usd_per_1m?: number // P11: contributor absolute price, USD per 1M tokens
}

export interface ContributorChannelsResponse {
  success: boolean
  message: string
  data: ContributorChannel[]
}

export interface ContributorChannelResponse {
  success: boolean
  message: string
  data: ContributorChannel
}

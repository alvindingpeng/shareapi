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
}

export interface ContributorChannelInput {
  type: number
  key: string
  name: string
  base_url?: string | null
  models: string
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

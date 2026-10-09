import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  ContributorChannelInput,
  ContributorChannelResponse,
  ContributorChannelsResponse,
} from './types'

const BASE = '/api/contributor/channels'

export async function listMyChannels(): Promise<ContributorChannelsResponse['data']> {
  const response = await api.get<ContributorChannelsResponse>(`${BASE}/`)
  requireServerSuccess(response.data)
  return response.data.data
}

export async function submitChannel(input: ContributorChannelInput): Promise<void> {
  const response = await api.post<{ success: boolean; message: string }>(`${BASE}/`, input)
  requireServerSuccess(response.data)
}

export async function updateMyChannel(
  id: number,
  input: Partial<ContributorChannelInput>,
): Promise<void> {
  const response = await api.put<{ success: boolean; message: string }>(`${BASE}/${id}`, input)
  requireServerSuccess(response.data)
}

export async function deleteMyChannel(id: number): Promise<void> {
  const response = await api.delete<{ success: boolean; message: string }>(`${BASE}/${id}`)
  requireServerSuccess(response.data)
}

export async function rotateMyChannelKey(id: number, key: string): Promise<void> {
  const response = await api.post<{ success: boolean; message: string }>(
    `${BASE}/${id}/rotate-key`,
    { key },
  )
  requireServerSuccess(response.data)
}

export type { ContributorChannelResponse }

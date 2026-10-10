/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StatusBadge } from '@/components/status-badge'
import { listMarketplaceChannels } from '@/features/marketplace/api'
import type { MarketplaceChannel } from '@/features/marketplace/types'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getChannelAllowlist, updateChannelAllowlist } from '../../api'
import type { ApiKey } from '../../types'

interface ChannelAllowlistDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: ApiKey | null
}

function trustScoreVariant(score: number): 'success' | 'warning' | 'danger' {
  if (score >= 70) return 'success'
  if (score >= 40) return 'warning'
  return 'danger'
}

export function ChannelAllowlistDialog(props: ChannelAllowlistDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [search, setSearch] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const channelsQuery = useQuery({
    queryKey: ['marketplace-channels', 'allowlist'],
    queryFn: () => listMarketplaceChannels('trust'),
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })

  const allowlistQuery = useQuery({
    queryKey: ['token-channel-allowlist', props.apiKey?.id],
    queryFn: async () => {
      if (!props.apiKey) return []
      const res = await getChannelAllowlist(props.apiKey.id)
      return requireServerSuccess(res).data?.channel_ids ?? []
    },
    enabled: props.open && props.apiKey != null,
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    if (props.open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelected(new Set(allowlistQuery.data ?? []))
      setSearch('')
    }
  }, [props.open, allowlistQuery.data])

  const channels = useMemo(
    () => channelsQuery.data ?? [],
    [channelsQuery.data]
  )

  const filteredChannels = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return channels
    return channels.filter(
      (ch) =>
        ch.name.toLowerCase().includes(keyword) ||
        ch.sharer.toLowerCase().includes(keyword)
    )
  }, [channels, search])

  const toggleChannel = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const handleSelectAll = () => {
    setSelected(new Set(filteredChannels.map((ch) => ch.channel_id)))
  }

  const handleClear = () => {
    setSelected(new Set())
  }

  const handleSave = async () => {
    if (!props.apiKey) return
    setIsSaving(true)
    try {
      const res = await updateChannelAllowlist(
        props.apiKey.id,
        [...selected]
      )
      requireServerSuccess(res)
      toast.success(t('Channel allowlist updated'))
      queryClient.invalidateQueries({ queryKey: ['keys'] })
      queryClient.invalidateQueries({
        queryKey: ['token-channel-allowlist', props.apiKey.id],
      })
      props.onOpenChange(false)
    } catch (error) {
      handleServerError(error, t('Failed to update channel allowlist'))
    } finally {
      setIsSaving(false)
    }
  }

  const renderBody = () => {
    if (channelsQuery.isLoading || allowlistQuery.isLoading) {
      return <LoadingState />
    }
    if (channelsQuery.isError) {
      return (
        <ErrorState
          title={t('Failed to load channels')}
          onRetry={() => channelsQuery.refetch()}
        />
      )
    }
    if (channels.length === 0) {
      return <EmptyState title={t('No available channels')} />
    }
    return (
      <div className='space-y-3'>
        <div className='flex items-center gap-2'>
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Search channels or sharers')}
            className='h-8'
          />
          <Button variant='outline' size='sm' onClick={handleSelectAll}>
            {t('Select all')}
          </Button>
          <Button variant='outline' size='sm' onClick={handleClear}>
            {t('Clear')}
          </Button>
        </div>
        <div className='text-muted-foreground text-xs'>
          {selected.size === 0
            ? t('No restriction')
            : t('{{count}} channels bound', { count: selected.size })}
        </div>
        <div className='max-h-[320px] space-y-1 overflow-y-auto pr-1'>
          {filteredChannels.map((ch: MarketplaceChannel) => (
            <Label
              key={ch.channel_id}
              htmlFor={`allowlist-channel-${ch.channel_id}`}
              className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-md border px-3 py-2'
            >
              <Checkbox
                id={`allowlist-channel-${ch.channel_id}`}
                checked={selected.has(ch.channel_id)}
                onCheckedChange={() => toggleChannel(ch.channel_id)}
              />
              <span className='min-w-0 flex-1'>
                <span className='block truncate text-sm font-medium'>
                  {ch.name}
                </span>
                <span className='text-muted-foreground block truncate text-xs'>
                  {ch.sharer} ·{' '}
                  {ch.price_multiplier === 0
                    ? t('Free')
                    : `×${ch.price_multiplier}`}
                </span>
              </span>
              <StatusBadge
                label={String(ch.trust_score)}
                variant={trustScoreVariant(ch.trust_score)}
                copyable={false}
              />
            </Label>
          ))}
          {filteredChannels.length === 0 && (
            <EmptyState title={t('No channels match your search')} />
          )}
        </div>
      </div>
    )
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Bind Channels')}
      description={t(
        'Select which contributor channels this API key may use. Leave empty for no restriction.'
      )}
      contentClassName='sm:max-w-lg'
      contentHeight='auto'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave} disabled={isSaving}>
            {isSaving ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      {renderBody()}
    </Dialog>
  )
}

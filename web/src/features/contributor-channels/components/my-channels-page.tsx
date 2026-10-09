import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { LoadingState } from '@/components/loading-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  CHANNEL_STATUS,
  CHANNEL_STATUS_CONFIG,
} from '@/features/channels/constants'
import { handleServerError } from '@/lib/handle-server-error'

import { deleteMyChannel, listMyChannels } from '../api'
import type { ContributorChannel } from '../types'
import { SubmitChannelDialog } from './submit-channel-dialog'

function StatusCell(props: { status: number }) {
  const { t } = useTranslation()
  const config =
    CHANNEL_STATUS_CONFIG[props.status as keyof typeof CHANNEL_STATUS_CONFIG] ??
    CHANNEL_STATUS_CONFIG[CHANNEL_STATUS.UNKNOWN]
  return <StatusBadge variant={config.variant} label={t(config.label)} size="sm" />
}

// Verification badge (Phase 3 anti-dilution trust signal).
function VerificationCell(props: { status: number }) {
  const { t } = useTranslation()
  switch (props.status) {
    case 1:
      return <StatusBadge variant="success" label={t('Verified')} size="sm" />
    case 2:
      return <StatusBadge variant="warning" label={t('Suspicious')} size="sm" />
    default:
      return <StatusBadge variant="neutral" label={t('Unverified')} size="sm" />
  }
}

function ChannelRow(props: {
  channel: ContributorChannel
  onDelete: (channel: ContributorChannel) => void
}) {
  const { t } = useTranslation()
  const channel = props.channel
  return (
    <TableRow>
      <TableCell className="font-medium">{channel.name}</TableCell>
      <TableCell>{channel.models || '—'}</TableCell>
      <TableCell>
        <StatusCell status={channel.status} />
      </TableCell>
      <TableCell>
        <VerificationCell status={channel.verification_status} />
      </TableCell>
      <TableCell className="text-right">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => props.onDelete(channel)}
        >
          {t('Delete')}
        </Button>
      </TableCell>
    </TableRow>
  )
}

export function MyChannelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [submitOpen, setSubmitOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ContributorChannel | null>(null)

  const channelsQuery = useQuery({
    queryKey: ['contributor-channels'],
    queryFn: listMyChannels,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteMyChannel(id),
    onSuccess: () => {
      setDeleteTarget(null)
      queryClient.invalidateQueries({ queryKey: ['contributor-channels'] })
    },
    onError: (error) => handleServerError(error, t('Failed to delete channel')),
  })

  const channels = channelsQuery.data ?? []

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t('My Channels')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Channels you contribute. New submissions go live after review.')}
          </p>
        </div>
        <Button onClick={() => setSubmitOpen(true)}>{t('Submit a channel')}</Button>
      </div>

      {channelsQuery.isLoading && <LoadingState />}
      {!channelsQuery.isLoading && channels.length === 0 && (
        <EmptyState
          title={t('No channels yet')}
          description={t('Submit your first API key channel to get started.')}
          action={
            <Button onClick={() => setSubmitOpen(true)}>{t('Submit a channel')}</Button>
          }
        />
      )}
      {!channelsQuery.isLoading && channels.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Name')}</TableHead>
              <TableHead>{t('Models')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Verification')}</TableHead>
              <TableHead className="text-right">{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {channels.map((channel) => (
              <ChannelRow
                key={channel.id}
                channel={channel}
                onDelete={setDeleteTarget}
              />
            ))}
          </TableBody>
        </Table>
      )}

      <SubmitChannelDialog
        open={submitOpen}
        onOpenChange={setSubmitOpen}
        onSubmitted={() =>
          queryClient.invalidateQueries({ queryKey: ['contributor-channels'] })
        }
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete channel?')}
        desc={t('This will permanently delete "{{name}}".', {
          name: deleteTarget?.name ?? '',
        })}
        confirmText={t('Delete')}
        destructive
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
        }}
        isLoading={deleteMutation.isPending}
      />
    </div>
  )
}

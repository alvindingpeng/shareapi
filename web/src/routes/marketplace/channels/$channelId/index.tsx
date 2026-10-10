import { createFileRoute } from '@tanstack/react-router'

import { ChannelDetailPage } from '@/features/marketplace/components/channel-detail-page'

export const Route = createFileRoute('/marketplace/channels/$channelId/')({
  component: RouteComponent,
})

function RouteComponent() {
  const { channelId } = Route.useParams()
  return <ChannelDetailPage channelId={Number(channelId)} />
}

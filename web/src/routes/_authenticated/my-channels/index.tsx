import { createFileRoute } from '@tanstack/react-router'

import { MyChannelsPage } from '@/features/contributor-channels/components/my-channels-page'

export const Route = createFileRoute('/_authenticated/my-channels/')({
  component: MyChannelsPage,
})

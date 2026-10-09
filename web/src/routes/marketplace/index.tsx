import { createFileRoute } from '@tanstack/react-router'

import { MarketplacePage } from '@/features/marketplace/components/marketplace-page'

export const Route = createFileRoute('/marketplace/')({
  component: MarketplacePage,
})

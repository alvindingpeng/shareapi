import { createFileRoute } from '@tanstack/react-router'

import { FundsPage } from '@/features/contributor-funds/components/funds-page'

export const Route = createFileRoute('/_authenticated/my-funds/')({
  component: FundsPage,
})

import { createFileRoute } from '@tanstack/react-router'

import { WithdrawalReviewPage } from '@/features/withdrawal-review/components/withdrawal-review-page'

export const Route = createFileRoute('/_authenticated/withdrawals/')({
  component: WithdrawalReviewPage,
})

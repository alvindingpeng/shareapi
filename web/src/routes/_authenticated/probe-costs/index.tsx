import { createFileRoute, redirect } from '@tanstack/react-router'

import { ProbeCostsPage } from '@/features/probe-costs/components/probe-costs-page'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/probe-costs/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if ((auth.user?.role ?? ROLE.GUEST) < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: ProbeCostsPage,
})

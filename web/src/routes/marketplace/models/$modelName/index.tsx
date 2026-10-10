import { createFileRoute } from '@tanstack/react-router'

import { ModelDetailPage } from '@/features/marketplace/components/model-detail-page'

export const Route = createFileRoute('/marketplace/models/$modelName/')({
  component: RouteComponent,
})

function RouteComponent() {
  // TanStack Router percent-decodes path params, so model names containing
  // characters like `/` (encoded as %2F by Link) arrive decoded here.
  const { modelName } = Route.useParams()
  return <ModelDetailPage modelName={modelName} />
}

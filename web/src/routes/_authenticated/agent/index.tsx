import { createFileRoute } from '@tanstack/react-router'

import { AgentConsole } from '@/features/agent'

export const Route = createFileRoute('/_authenticated/agent/')({
  component: AgentConsole,
})

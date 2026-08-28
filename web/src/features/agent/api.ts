import { api } from '@/lib/api'

import type {
  AgentChild,
  AgentChildPricingRow,
  AgentDomain,
  AgentLedgerRow,
  AgentPricingRow,
  AgentSelf,
  AgentType,
  AgentWithdrawRow,
} from './types'

interface Envelope<T> {
  success: boolean
  message: string
  data: T
}

export async function getSelfAgent(): Promise<{
  is_agent: boolean
  agent?: AgentSelf
}> {
  const res =
    await api.get<Envelope<{ is_agent: boolean; agent?: AgentSelf }>>(
      '/api/agent/self'
    )
  return res.data.data
}

export async function getChildAgents(): Promise<AgentChild[]> {
  const res = await api.get<Envelope<AgentChild[]>>('/api/agent/children')
  return res.data.data ?? []
}

export async function createSubAgent(payload: {
  user_id: number
  type: AgentType
  name: string
}) {
  const res = await api.post<Envelope<AgentSelf>>(
    '/api/agent/children',
    payload
  )
  return res.data
}

export async function getSelfPricing(): Promise<AgentPricingRow[]> {
  const res = await api.get<Envelope<AgentPricingRow[]>>('/api/agent/pricing')
  return res.data.data ?? []
}

export async function setSellRate(group: string, rate: number) {
  const res = await api.put<Envelope<null>>('/api/agent/pricing/sell', {
    group,
    rate,
  })
  return res.data
}

export async function getChildPricing(
  childId: number
): Promise<AgentChildPricingRow[]> {
  const res = await api.get<Envelope<AgentChildPricingRow[]>>(
    `/api/agent/children/${childId}/pricing`
  )
  return res.data.data ?? []
}

export async function setChildCostRate(
  childId: number,
  group: string,
  rate: number
) {
  const res = await api.put<Envelope<null>>(
    `/api/agent/children/${childId}/pricing/cost`,
    { group, rate }
  )
  return res.data
}

export async function getAgentDomains(): Promise<AgentDomain[]> {
  const res = await api.get<Envelope<AgentDomain[]>>('/api/agent/domains')
  return res.data.data ?? []
}

export async function bindAgentDomain(domain: string) {
  const res = await api.post<
    Envelope<{ id: number; domain: string; verify_token: string }>
  >('/api/agent/domains', { domain })
  return res.data
}

export async function verifyAgentDomain(id: number) {
  const res = await api.post<Envelope<null>>(`/api/agent/domains/${id}/verify`)
  return res.data
}

export async function updateAgentDomainBranding(
  id: number,
  payload: Partial<AgentDomain>
) {
  const res = await api.put<Envelope<null>>(
    `/api/agent/domains/${id}/branding`,
    payload
  )
  return res.data
}

export async function unbindAgentDomain(id: number) {
  const res = await api.delete<Envelope<null>>(`/api/agent/domains/${id}`)
  return res.data
}

export async function getAgentLedger(page = 1): Promise<AgentLedgerRow[]> {
  const res = await api.get<Envelope<AgentLedgerRow[]>>(
    `/api/agent/ledger?page=${page}`
  )
  return res.data.data ?? []
}

export async function getAgentWithdraws(page = 1): Promise<AgentWithdrawRow[]> {
  const res = await api.get<Envelope<AgentWithdrawRow[]>>(
    `/api/agent/withdraw?page=${page}`
  )
  return res.data.data ?? []
}

export async function createAgentWithdraw(quota: number, payeeInfo: string) {
  const res = await api.post<Envelope<AgentWithdrawRow>>(
    '/api/agent/withdraw',
    { quota, payee_info: payeeInfo }
  )
  return res.data
}

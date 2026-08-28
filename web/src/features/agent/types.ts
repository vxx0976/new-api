export type AgentType = 'reseller' | 'affiliate'

export const AGENT_STATUS = {
  ACTIVE: 1,
  FROZEN: 2,
  SUSPENDED: 3,
} as const

export interface AgentSelf {
  id: number
  parent_agent_id: number
  level: number
  type: AgentType
  name: string
  status: number
  earning_quota: number
  history_earning_quota: number
  withdrawn_quota: number
  rebate_rate_percent: number
  created_at: number
}

/**
 * 直接下级。刻意不含下级的售价与下级的下级数量——
 * 逐级批发模型下上级无权知晓这两项，后端也不会返回。
 */
export interface AgentChild {
  id: number
  owner_user_id: number
  owner_username: string
  name: string
  type: AgentType
  status: number
  rebate_rate_percent: number
  created_at: number
}

export interface AgentPricingRow {
  group: string
  cost_rate: number
  sell_rate: number
  pending_cost_rate: number
  pending_effective_at: number
}

export interface AgentChildPricingRow {
  group: string
  cost_rate: number
  pending_cost_rate: number
  pending_effective_at: number
}

export interface AgentDomain {
  id: number
  agent_id: number
  domain: string
  site_name: string
  site_logo: string
  brand_color: string
  home_content: string
  verify_token: string
  verified: boolean
  verified_at: number
}

export interface AgentLedgerRow {
  id: number
  direction: 'credit' | 'debit'
  amount: number
  balance_after: number
  source: string
  counterparty_user_id: number
  created_at: number
}

export interface AgentWithdrawRow {
  id: number
  quota: number
  status: number
  payee_info: string
  review_note: string
  created_at: number
}

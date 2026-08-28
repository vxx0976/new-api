import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { getSelfAgent } from './api'
import { AgentChildrenPanel } from './components/agent-children-panel'
import { AgentDomainsPanel } from './components/agent-domains-panel'
import { AgentPricingPanel } from './components/agent-pricing-panel'
import { AgentWalletPanel } from './components/agent-wallet-panel'
import { AGENT_STATUS, type AgentSelf } from './types'

export function AgentConsole() {
  const { t } = useTranslation()
  const [agent, setAgent] = useState<AgentSelf | null>(null)
  const [isAgent, setIsAgent] = useState<boolean | null>(null)

  const reload = useCallback(async () => {
    try {
      const data = await getSelfAgent()
      setIsAgent(data.is_agent)
      setAgent(data.agent ?? null)
    } catch (error) {
      toast.error(String(error))
      setIsAgent(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  if (isAgent === null) {
    return null
  }

  if (!isAgent || !agent) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Agent Console')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <Card>
            <CardHeader>
              <CardTitle>{t('Not an agent')}</CardTitle>
              <CardDescription>
                {t(
                  'Your account is not an agent yet. Contact your upstream agent to be enrolled.'
                )}
              </CardDescription>
            </CardHeader>
          </Card>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const frozen = agent.status === AGENT_STATUS.FROZEN
  const isReseller = agent.type === 'reseller'

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Agent Console')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={() => void reload()}>
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <Card>
            <CardHeader>
              <CardTitle>{agent.name}</CardTitle>
              <CardDescription>
                {isReseller ? t('Reseller') : t('Affiliate')} · {t('Tier')}{' '}
                {agent.level + 1}
                {frozen ? ` · ${t('Frozen')}` : ''}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className='grid grid-cols-1 gap-4 sm:grid-cols-3'>
                <Stat
                  label={t('Withdrawable earnings')}
                  value={formatQuotaWithCurrency(agent.earning_quota)}
                />
                <Stat
                  label={t('Total earned')}
                  value={formatQuotaWithCurrency(agent.history_earning_quota)}
                />
                <Stat
                  label={t('Total withdrawn')}
                  value={formatQuotaWithCurrency(agent.withdrawn_quota)}
                />
              </div>
            </CardContent>
          </Card>

          <Tabs defaultValue='children'>
            <TabsList>
              <TabsTrigger value='children'>
                {t('Direct downstream')}
              </TabsTrigger>
              {isReseller && (
                <TabsTrigger value='pricing'>{t('My pricing')}</TabsTrigger>
              )}
              {isReseller && (
                <TabsTrigger value='domains'>{t('Site domains')}</TabsTrigger>
              )}
              <TabsTrigger value='wallet'>{t('Earnings')}</TabsTrigger>
            </TabsList>

            <TabsContent value='children'>
              <AgentChildrenPanel
                agent={agent}
                onChanged={() => void reload()}
              />
            </TabsContent>
            {isReseller && (
              <TabsContent value='pricing'>
                <AgentPricingPanel />
              </TabsContent>
            )}
            {isReseller && (
              <TabsContent value='domains'>
                <AgentDomainsPanel />
              </TabsContent>
            )}
            <TabsContent value='wallet'>
              <AgentWalletPanel agent={agent} onChanged={() => void reload()} />
            </TabsContent>
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className='flex flex-col gap-1'>
      <span className='text-muted-foreground text-sm'>{label}</span>
      <span className='text-2xl font-semibold'>{value}</span>
    </div>
  )
}

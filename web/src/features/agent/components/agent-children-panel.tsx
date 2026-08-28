import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import {
  createSubAgent,
  getChildAgents,
  getChildPricing,
  setChildCostRate,
} from '../api'
import {
  AGENT_STATUS,
  type AgentChild,
  type AgentChildPricingRow,
  type AgentSelf,
  type AgentType,
} from '../types'

interface Props {
  agent: AgentSelf
  onChanged: () => void
}

export function AgentChildrenPanel({ agent, onChanged }: Props) {
  const { t } = useTranslation()
  const [children, setChildren] = useState<AgentChild[]>([])
  const [selected, setSelected] = useState<AgentChild | null>(null)
  const [pricing, setPricing] = useState<AgentChildPricingRow[]>([])
  const [userId, setUserId] = useState('')
  const [name, setName] = useState('')
  const [type, setType] = useState<AgentType>('reseller')
  const [group, setGroup] = useState('default')
  const [rate, setRate] = useState('')

  const reload = useCallback(async () => {
    try {
      setChildren(await getChildAgents())
    } catch (error) {
      toast.error(String(error))
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const openPricing = async (child: AgentChild) => {
    setSelected(child)
    try {
      setPricing(await getChildPricing(child.id))
    } catch (error) {
      toast.error(String(error))
    }
  }

  const handleCreate = async () => {
    const id = Number(userId)
    if (!id || !name.trim()) {
      toast.error(t('Please fill in the user ID and agent name'))
      return
    }
    const res = await createSubAgent({ user_id: id, type, name: name.trim() })
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Downstream agent created'))
    setUserId('')
    setName('')
    await reload()
    onChanged()
  }

  const handleSetCost = async () => {
    if (!selected) return
    const value = Number(rate)
    if (!value || value <= 0) {
      toast.error(t('Rate must be greater than 0'))
      return
    }
    const res = await setChildCostRate(selected.id, group, value)
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Wholesale rate saved'))
    setRate('')
    setPricing(await getChildPricing(selected.id))
  }

  const canOpenReseller = agent.type === 'reseller'

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Direct downstream')}</CardTitle>
          <CardDescription>
            {t(
              'You only see and manage your direct downstream. Their own downstream is not visible to you.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Owner')}</TableHead>
                <TableHead>{t('Type')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {children.map((child) => (
                <TableRow key={child.id}>
                  <TableCell>{child.name}</TableCell>
                  <TableCell>{child.owner_username}</TableCell>
                  <TableCell>
                    {child.type === 'reseller' ? t('Reseller') : t('Affiliate')}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        child.status === AGENT_STATUS.ACTIVE
                          ? 'default'
                          : 'secondary'
                      }
                    >
                      {child.status === AGENT_STATUS.ACTIVE
                        ? t('Active')
                        : t('Frozen')}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => void openPricing(child)}
                    >
                      {t('Wholesale price')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {children.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className='text-muted-foreground'>
                    {t('No downstream agents yet')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Enroll a downstream agent')}</CardTitle>
          <CardDescription>
            {t(
              'The target must already be one of your own users. Enrolling someone outside your tenant is rejected.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3 sm:flex-row sm:items-end'>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='agent-user-id'>{t('User ID')}</Label>
            <Input
              id='agent-user-id'
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='agent-name'>{t('Agent name')}</Label>
            <Input
              id='agent-name'
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='agent-type'>{t('Type')}</Label>
            <select
              id='agent-type'
              className='border-input h-9 rounded-md border bg-transparent px-3 text-sm'
              value={type}
              onChange={(e) => setType(e.target.value as AgentType)}
            >
              {canOpenReseller && (
                <option value='reseller'>{t('Reseller')}</option>
              )}
              <option value='affiliate'>{t('Affiliate')}</option>
            </select>
          </div>
          <Button onClick={() => void handleCreate()}>{t('Enroll')}</Button>
        </CardContent>
      </Card>

      {selected && (
        <Card>
          <CardHeader>
            <CardTitle>
              {t('Wholesale price for')} {selected.name}
            </CardTitle>
            <CardDescription>
              {t(
                'Price cuts take effect immediately. Price raises take effect after a delay so the downstream can adjust its own selling price first.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex flex-col gap-3'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Group')}</TableHead>
                  <TableHead>{t('Wholesale rate')}</TableHead>
                  <TableHead>{t('Pending')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pricing.map((row) => (
                  <TableRow key={row.group}>
                    <TableCell>{row.group}</TableCell>
                    <TableCell>{row.cost_rate}</TableCell>
                    <TableCell>
                      {row.pending_effective_at > 0
                        ? `${row.pending_cost_rate} @ ${new Date(
                            row.pending_effective_at * 1000
                          ).toLocaleString()}`
                        : '-'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
              <div className='flex flex-col gap-1'>
                <Label htmlFor='child-group'>{t('Group')}</Label>
                <Input
                  id='child-group'
                  value={group}
                  onChange={(e) => setGroup(e.target.value)}
                />
              </div>
              <div className='flex flex-col gap-1'>
                <Label htmlFor='child-rate'>{t('Wholesale rate')}</Label>
                <Input
                  id='child-rate'
                  value={rate}
                  onChange={(e) => setRate(e.target.value)}
                />
              </div>
              <Button onClick={() => void handleSetCost()}>{t('Save')}</Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

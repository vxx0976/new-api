import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { formatQuotaWithCurrency } from '@/lib/currency'

import { createAgentWithdraw, getAgentLedger, getAgentWithdraws } from '../api'
import type { AgentLedgerRow, AgentSelf, AgentWithdrawRow } from '../types'

const WITHDRAW_STATUS_KEY: Record<number, string> = {
  1: 'Pending review',
  2: 'Approved',
  3: 'Paid',
  4: 'Rejected',
}

interface Props {
  agent: AgentSelf
  onChanged: () => void
}

export function AgentWalletPanel({ agent, onChanged }: Props) {
  const { t } = useTranslation()
  const [ledger, setLedger] = useState<AgentLedgerRow[]>([])
  const [withdraws, setWithdraws] = useState<AgentWithdrawRow[]>([])
  const [quota, setQuota] = useState('')
  const [payee, setPayee] = useState('')

  const reload = useCallback(async () => {
    try {
      const [ledgerRows, withdrawRows] = await Promise.all([
        getAgentLedger(),
        getAgentWithdraws(),
      ])
      setLedger(ledgerRows)
      setWithdraws(withdrawRows)
    } catch (error) {
      toast.error(String(error))
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const handleWithdraw = async () => {
    const value = Number(quota)
    if (!value || value <= 0) {
      toast.error(t('Amount must be greater than 0'))
      return
    }
    const res = await createAgentWithdraw(value, payee)
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Withdrawal requested'))
    setQuota('')
    await reload()
    onChanged()
  }

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Request a withdrawal')}</CardTitle>
          <CardDescription>
            {t(
              'Earnings are a separate wallet from your API balance. Requesting locks the amount until the platform reviews it; your upstream is not involved.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3 sm:flex-row sm:items-end'>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='withdraw-quota'>{t('Amount (quota)')}</Label>
            <Input
              id='withdraw-quota'
              value={quota}
              onChange={(e) => setQuota(e.target.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='withdraw-payee'>{t('Payee info')}</Label>
            <Input
              id='withdraw-payee'
              value={payee}
              onChange={(e) => setPayee(e.target.value)}
            />
          </div>
          <Button onClick={() => void handleWithdraw()}>{t('Request')}</Button>
          <span className='text-muted-foreground text-sm'>
            {t('Available')}: {formatQuotaWithCurrency(agent.earning_quota)}
          </span>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Withdrawal history')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Note')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {withdraws.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>{formatQuotaWithCurrency(row.quota)}</TableCell>
                  <TableCell>
                    {t(WITHDRAW_STATUS_KEY[row.status] ?? 'Unknown')}
                  </TableCell>
                  <TableCell>{row.review_note || '-'}</TableCell>
                  <TableCell>
                    {new Date(row.created_at * 1000).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
              {withdraws.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className='text-muted-foreground'>
                    {t('No withdrawals yet')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Earnings ledger')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Direction')}</TableHead>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Balance after')}</TableHead>
                <TableHead>{t('Source')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {ledger.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>
                    {row.direction === 'credit' ? t('Credit') : t('Debit')}
                  </TableCell>
                  <TableCell>{formatQuotaWithCurrency(row.amount)}</TableCell>
                  <TableCell>
                    {formatQuotaWithCurrency(row.balance_after)}
                  </TableCell>
                  <TableCell>{row.source}</TableCell>
                  <TableCell>
                    {new Date(row.created_at * 1000).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
              {ledger.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className='text-muted-foreground'>
                    {t('No earnings yet')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}

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

import { getSelfPricing, setSellRate } from '../api'
import type { AgentPricingRow } from '../types'

export function AgentPricingPanel() {
  const { t } = useTranslation()
  const [rows, setRows] = useState<AgentPricingRow[]>([])
  const [group, setGroup] = useState('default')
  const [rate, setRate] = useState('')

  const reload = useCallback(async () => {
    try {
      setRows(await getSelfPricing())
    } catch (error) {
      toast.error(String(error))
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const handleSave = async () => {
    const value = Number(rate)
    if (!value || value <= 0) {
      toast.error(t('Rate must be greater than 0'))
      return
    }
    const res = await setSellRate(group, value)
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Selling price saved'))
    setRate('')
    await reload()
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('My pricing')}</CardTitle>
        <CardDescription>
          {t(
            'Your wholesale rate is set by your upstream and is read-only. Your selling price is yours to set and must not fall below your wholesale rate.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Group')}</TableHead>
              <TableHead>{t('Wholesale rate')}</TableHead>
              <TableHead>{t('Selling rate')}</TableHead>
              <TableHead>{t('Margin')}</TableHead>
              <TableHead>{t('Pending')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.group}>
                <TableCell>{row.group}</TableCell>
                <TableCell>{row.cost_rate}</TableCell>
                <TableCell>{row.sell_rate || '-'}</TableCell>
                <TableCell>
                  {row.sell_rate
                    ? (row.sell_rate - row.cost_rate).toFixed(6)
                    : '-'}
                </TableCell>
                <TableCell>
                  {row.pending_effective_at > 0
                    ? `${row.pending_cost_rate} @ ${new Date(
                        row.pending_effective_at * 1000
                      ).toLocaleString()}`
                    : '-'}
                </TableCell>
              </TableRow>
            ))}
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className='text-muted-foreground'>
                  {t('No pricing configured yet')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='sell-group'>{t('Group')}</Label>
            <Input
              id='sell-group'
              value={group}
              onChange={(e) => setGroup(e.target.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label htmlFor='sell-rate'>{t('Selling rate')}</Label>
            <Input
              id='sell-rate'
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </div>
          <Button onClick={() => void handleSave()}>{t('Save')}</Button>
        </div>
      </CardContent>
    </Card>
  )
}

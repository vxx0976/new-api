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
  bindAgentDomain,
  getAgentDomains,
  unbindAgentDomain,
  updateAgentDomainBranding,
  verifyAgentDomain,
} from '../api'
import type { AgentDomain } from '../types'

export function AgentDomainsPanel() {
  const { t } = useTranslation()
  const [domains, setDomains] = useState<AgentDomain[]>([])
  const [domain, setDomain] = useState('')
  const [editing, setEditing] = useState<AgentDomain | null>(null)
  const [siteName, setSiteName] = useState('')
  const [siteLogo, setSiteLogo] = useState('')

  const reload = useCallback(async () => {
    try {
      setDomains(await getAgentDomains())
    } catch (error) {
      toast.error(String(error))
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const handleBind = async () => {
    if (!domain.trim()) return
    const res = await bindAgentDomain(domain.trim())
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Domain bound. Add the TXT record, then verify.'))
    setDomain('')
    await reload()
  }

  const handleVerify = async (row: AgentDomain) => {
    const res = await verifyAgentDomain(row.id)
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Domain verified'))
    await reload()
  }

  const handleUnbind = async (row: AgentDomain) => {
    const res = await unbindAgentDomain(row.id)
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Domain unbound'))
    await reload()
  }

  const handleSaveBranding = async () => {
    if (!editing) return
    const res = await updateAgentDomainBranding(editing.id, {
      site_name: siteName,
      site_logo: siteLogo,
    })
    if (!res.success) {
      toast.error(res.message)
      return
    }
    toast.success(t('Branding saved'))
    setEditing(null)
    await reload()
  }

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Site domains')}</CardTitle>
          <CardDescription>
            {t(
              'Point your domain here with a CNAME, then prove ownership with the TXT record. Unverified domains never take effect.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Domain')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('TXT record')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {domains.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>{row.domain}</TableCell>
                  <TableCell>
                    <Badge variant={row.verified ? 'default' : 'secondary'}>
                      {row.verified ? t('Verified') : t('Pending verification')}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {row.verified ? '-' : row.verify_token}
                  </TableCell>
                  <TableCell className='flex justify-end gap-2'>
                    {!row.verified && (
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => void handleVerify(row)}
                      >
                        {t('Verify')}
                      </Button>
                    )}
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => {
                        setEditing(row)
                        setSiteName(row.site_name)
                        setSiteLogo(row.site_logo)
                      }}
                    >
                      {t('Branding')}
                    </Button>
                    <Button
                      size='sm'
                      variant='destructive'
                      onClick={() => void handleUnbind(row)}
                    >
                      {t('Unbind')}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {domains.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className='text-muted-foreground'>
                    {t('No domains bound yet')}
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className='mt-3 flex flex-col gap-3 sm:flex-row sm:items-end'>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='new-domain'>{t('Domain')}</Label>
              <Input
                id='new-domain'
                placeholder='api.example.com'
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
              />
            </div>
            <Button onClick={() => void handleBind()}>{t('Bind')}</Button>
          </div>
        </CardContent>
      </Card>

      {editing && (
        <Card>
          <CardHeader>
            <CardTitle>
              {t('Branding')} · {editing.domain}
            </CardTitle>
          </CardHeader>
          <CardContent className='flex flex-col gap-3 sm:flex-row sm:items-end'>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='brand-site-name'>{t('Site name')}</Label>
              <Input
                id='brand-site-name'
                value={siteName}
                onChange={(e) => setSiteName(e.target.value)}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='brand-logo'>{t('Logo URL')}</Label>
              <Input
                id='brand-logo'
                value={siteLogo}
                onChange={(e) => setSiteLogo(e.target.value)}
              />
            </div>
            <Button onClick={() => void handleSaveBranding()}>
              {t('Save')}
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

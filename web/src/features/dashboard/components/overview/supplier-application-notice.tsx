/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Clock, XCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SUPPLIER_STATUS } from '@/features/users/constants'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Applicants have no other way to learn where their supplier application
 * stands: approval only shows up as a new menu entry, and a rejection is
 * silent. This states the outcome on the page they land on after signing in.
 */
export function SupplierApplicationNotice() {
  const { t } = useTranslation()
  const status = useAuthStore((state) => state.auth.user?.supplier_status)

  if (
    status !== SUPPLIER_STATUS.PENDING &&
    status !== SUPPLIER_STATUS.REJECTED
  ) {
    return null
  }

  const pending = status === SUPPLIER_STATUS.PENDING
  return (
    <div
      className={cn(
        'flex items-start gap-3 rounded-lg border p-4',
        pending
          ? 'border-warning/40 bg-warning/5'
          : 'border-destructive/40 bg-destructive/5'
      )}
    >
      {pending ? (
        <Clock className='text-warning mt-0.5 size-4 shrink-0' />
      ) : (
        <XCircle className='text-destructive mt-0.5 size-4 shrink-0' />
      )}
      <div className='min-w-0'>
        <p className='text-sm font-medium'>
          {pending
            ? t('Your supplier application is under review')
            : t('Your supplier application was not approved')}
        </p>
        <p className='text-muted-foreground mt-1 text-sm leading-relaxed'>
          {pending
            ? t(
                'You can use the platform as a regular user in the meantime. Once approved, "My Channels" appears in the sidebar and you can list channels.'
              )
            : t(
                'Contact the platform administrator if you would like your application reconsidered.'
              )}
        </p>
      </div>
    </div>
  )
}

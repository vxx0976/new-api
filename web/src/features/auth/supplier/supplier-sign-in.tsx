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
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import { UserAuthForm } from '../sign-in/components/user-auth-form'
import { SupplierAuthHeading } from './supplier-auth-heading'

export function SupplierSignIn() {
  const { t } = useTranslation()
  const { redirect } = useSearch({ from: '/(auth)/supplier-sign-in' })
  const { status } = useStatus()

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <SupplierAuthHeading title={t('Supplier sign in')}>
          {t('Want to list your models here?')}{' '}
          <Link
            to='/supplier-sign-up'
            className='hover:text-primary font-medium underline underline-offset-4'
          >
            {t('Become a supplier')}
          </Link>
          .
        </SupplierAuthHeading>

        <UserAuthForm redirectTo={redirect} />

        <TermsFooter
          variant='sign-in'
          status={status}
          className='text-center'
        />
      </div>
    </AuthLayout>
  )
}

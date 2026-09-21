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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { SUPPLIER_STATUS } from '@/features/users/constants'
import { useAuthStore } from '@/stores/auth-store'

import { SupplierApplicationNotice } from '../supplier-application-notice'

afterEach(cleanup)

function renderForStatus(supplier_status?: number) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'applicant',
    role: 1,
    supplier_status,
  })
  render(<SupplierApplicationNotice />)
}

test('a pending applicant is told the application is still under review', () => {
  renderForStatus(SUPPLIER_STATUS.PENDING)
  expect(screen.getByText(/under review/i)).toBeInTheDocument()
})

test('a rejected applicant is told the application was not approved', () => {
  renderForStatus(SUPPLIER_STATUS.REJECTED)
  expect(screen.getByText(/not approved/i)).toBeInTheDocument()
})

test('nothing is shown to users with no application, or once approved', () => {
  for (const status of [
    undefined,
    SUPPLIER_STATUS.NONE,
    SUPPLIER_STATUS.APPROVED,
  ]) {
    cleanup()
    renderForStatus(status)
    expect(screen.queryByText(/under review/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/not approved/i)).not.toBeInTheDocument()
  }
})

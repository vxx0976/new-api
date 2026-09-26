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
import { afterEach, beforeEach, expect, test } from 'vitest'

import { ThemeProvider } from '@/context/theme-provider'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { BrandLockup } from '../brand-lockup'

const LIGHT = '/logo-wide.svg'
const DARK = '/logo-wide-dark.svg'

beforeEach(() => {
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
})

afterEach(cleanup)

function configure(config: { logoWide?: string; logoWideDark?: string }) {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, systemName: '优汇采', ...config },
  }))
}

function renderIn(theme: 'light' | 'dark') {
  render(
    <ThemeProvider defaultTheme={theme} storageKey={`brand-lockup-${theme}`}>
      <BrandLockup fallback={<span>logo and site name</span>} />
    </ThemeProvider>
  )
}

test('without a configured lockup the existing logo and site name are shown', () => {
  configure({})
  renderIn('light')

  expect(screen.getByText('logo and site name')).toBeInTheDocument()
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
})

test('a configured lockup replaces the logo and site name and is named after the site', () => {
  configure({ logoWide: LIGHT })
  renderIn('light')

  expect(screen.getByRole('img', { name: '优汇采' })).toHaveAttribute(
    'src',
    LIGHT
  )
  expect(screen.queryByText('logo and site name')).not.toBeInTheDocument()
})

test('dark mode uses the dark lockup when one is configured', () => {
  configure({ logoWide: LIGHT, logoWideDark: DARK })
  renderIn('dark')

  expect(screen.getByRole('img', { name: '优汇采' })).toHaveAttribute(
    'src',
    DARK
  )
})

test('dark mode falls back to the light lockup when no dark one is configured', () => {
  configure({ logoWide: LIGHT })
  renderIn('dark')

  expect(screen.getByRole('img', { name: '优汇采' })).toHaveAttribute(
    'src',
    LIGHT
  )
})

test('a dark lockup alone is not used in light mode', () => {
  configure({ logoWideDark: DARK })
  renderIn('light')

  expect(screen.getByText('logo and site name')).toBeInTheDocument()
})

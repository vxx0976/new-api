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
import {
  BadgeCheck,
  Boxes,
  KeyRound,
  ListChecks,
  Store,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

const BUYER_POINTS = [
  { icon: KeyRound, textKey: 'One API key works across every listed model' },
  { icon: Wallet, textKey: 'Pay as you go with transparent per-model pricing' },
  {
    icon: Boxes,
    textKey: 'Requests are routed across suppliers automatically',
  },
]

const SUPPLIER_POINTS = [
  {
    icon: BadgeCheck,
    textKey: 'Register with your company and pass platform review',
  },
  {
    icon: ListChecks,
    textKey: 'List channels under the standard names of each channel type',
  },
  {
    icon: Store,
    textKey: 'Manage, test and take your own channels offline at any time',
  },
]

export function MarketplaceRoles() {
  const { t } = useTranslation()
  const columns = [
    { titleKey: 'For developers and teams', points: BUYER_POINTS },
    { titleKey: 'For suppliers', points: SUPPLIER_POINTS },
  ]

  return (
    <section className='relative z-10 px-6 py-16 md:py-20'>
      <div className='mx-auto max-w-6xl'>
        <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
          {t('Built for both sides of the marketplace')}
        </h2>
        <div className='mt-8 grid gap-6 md:grid-cols-2'>
          {columns.map((column) => (
            <div
              key={column.titleKey}
              className='bg-card rounded-2xl border p-6'
            >
              <h3 className='text-base font-semibold'>{t(column.titleKey)}</h3>
              <ul className='mt-4 space-y-3'>
                {column.points.map((point) => (
                  <li key={point.textKey} className='flex items-start gap-3'>
                    <span className='bg-primary/10 text-primary mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md'>
                      <point.icon className='size-4' aria-hidden='true' />
                    </span>
                    <span className='text-muted-foreground text-sm leading-relaxed'>
                      {t(point.textKey)}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

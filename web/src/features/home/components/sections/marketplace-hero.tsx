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
import { Link } from '@tanstack/react-router'
import { ArrowRight, ShieldCheck, Store } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

interface MarketplaceHeroProps {
  isAuthenticated?: boolean
}

export function MarketplaceHero(props: MarketplaceHeroProps) {
  const { t } = useTranslation()
  const isSupplier = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPPLIER
  )

  return (
    <section className='relative z-10 overflow-hidden px-6 pt-24 pb-16 md:pt-32 md:pb-24'>
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 -z-10 opacity-25 dark:opacity-[0.12]'
        style={{
          background: [
            'radial-gradient(ellipse 60% 50% at 20% 20%, oklch(0.62 0.19 25 / 70%) 0%, transparent 70%)',
            'radial-gradient(ellipse 50% 40% at 80% 15%, oklch(0.72 0.14 70 / 55%) 0%, transparent 70%)',
          ].join(', '),
        }}
      />
      <div
        aria-hidden
        className='absolute inset-0 -z-10 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_30%,black_20%,transparent_100%)] bg-[size:4rem_4rem] opacity-[0.08]'
      />

      <div className='mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-10'>
        <div className='flex flex-col items-start text-left lg:col-span-7'>
          <div className='landing-animate-fade-up border-primary/20 bg-primary/5 text-primary mb-5 inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[11px] font-medium'>
            <ShieldCheck className='size-3.5' aria-hidden='true' />
            <span>{t('Platform-reviewed suppliers')}</span>
          </div>

          <h1 className='landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight'>
            {t('One marketplace for')}
            <br />
            <span className='from-primary bg-gradient-to-r to-amber-500 bg-clip-text text-transparent'>
              {t('AI model capacity from many suppliers')}
            </span>
          </h1>
          <p className='landing-animate-fade-up text-muted-foreground/80 mt-5 max-w-xl text-base leading-relaxed'>
            {t(
              'Buy tokens for leading AI models through one account and one API, supplied by vetted vendors who list and operate their own channels.'
            )}
          </p>

          <div className='landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3'>
            <Button
              className='group h-11 rounded-lg px-5 text-sm font-medium'
              render={
                <Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />
              }
            >
              {props.isAuthenticated ? t('Go to Dashboard') : t('Get Started')}
              <ArrowRight className='ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
            </Button>
            <Button
              variant='outline'
              className='border-border/50 hover:border-border hover:bg-muted/50 h-11 rounded-lg px-5 text-sm font-medium'
              render={<Link to='/pricing' />}
            >
              {t('View Pricing')}
            </Button>
          </div>
        </div>

        <aside className='landing-animate-fade-up bg-card/80 rounded-2xl border p-6 shadow-sm backdrop-blur lg:col-span-5'>
          <span className='bg-primary/10 text-primary flex size-11 items-center justify-center rounded-xl'>
            <Store className='size-5' aria-hidden='true' />
          </span>
          <h2 className='mt-4 text-lg font-semibold'>
            {t('Sell your model capacity')}
          </h2>
          <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
            {t(
              'Apply with your company name, get approved by the platform, then list and manage your own channels.'
            )}
          </p>
          <div className='mt-5 flex flex-wrap items-center gap-3'>
            {isSupplier ? (
              <Button
                className='h-10 rounded-lg px-4 text-sm font-medium'
                render={<Link to='/channels' />}
              >
                {t('My Channels')}
              </Button>
            ) : (
              <>
                <Button
                  className='h-10 rounded-lg px-4 text-sm font-medium'
                  render={<Link to='/supplier-sign-up' />}
                >
                  {t('Become a supplier')}
                </Button>
                <Button
                  variant='ghost'
                  className='h-10 rounded-lg px-4 text-sm font-medium'
                  render={<Link to='/supplier-sign-in' />}
                >
                  {t('Supplier sign in')}
                </Button>
              </>
            )}
          </div>
        </aside>
      </div>
    </section>
  )
}

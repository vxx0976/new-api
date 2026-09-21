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
import { ArrowUp, BookOpen, Headset } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'

type RailButtonProps = {
  label: string
  detail?: string
  icon: React.ReactNode
  onClick?: () => void
  href?: string
}

function RailButton(props: RailButtonProps) {
  const className =
    'bg-card text-muted-foreground hover:text-primary hover:border-primary/40 border-border flex size-11 items-center justify-center border-b transition-colors first:rounded-t-md last:rounded-b-md last:border-b-0'
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          props.href ? (
            <a
              href={props.href}
              target='_blank'
              rel='noopener noreferrer'
              aria-label={props.label}
              className={className}
            />
          ) : (
            <button
              type='button'
              onClick={props.onClick}
              aria-label={props.label}
              className={className}
            />
          )
        }
      >
        {props.icon}
      </TooltipTrigger>
      <TooltipContent side='left'>
        <p>{props.label}</p>
        {props.detail && <p className='font-medium'>{props.detail}</p>}
      </TooltipContent>
    </Tooltip>
  )
}

/**
 * Fixed utility rail on the right edge: the service-contact affordance
 * enterprise and government portals in this market place there.
 */
export function ServiceRail() {
  const { t } = useTranslation()
  const { serviceHotline } = useSystemConfig()
  const { status } = useStatus()
  const [showBackToTop, setShowBackToTop] = useState(false)

  useEffect(() => {
    const onScroll = () => setShowBackToTop(window.scrollY > 400)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  const docsUrl = (status?.docs_link as string | undefined) || ''

  return (
    <div className='pointer-events-auto fixed top-1/2 right-3 z-40 hidden -translate-y-1/2 flex-col rounded-md border shadow-sm md:flex'>
      {serviceHotline && (
        <RailButton
          label={t('Service hotline')}
          detail={serviceHotline}
          icon={<Headset className='size-4' />}
        />
      )}
      {docsUrl && (
        <RailButton
          label={t('Docs')}
          href={docsUrl}
          icon={<BookOpen className='size-4' />}
        />
      )}
      {showBackToTop && (
        <RailButton
          label={t('Back to top')}
          onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}
          icon={<ArrowUp className='size-4' />}
        />
      )}
    </div>
  )
}

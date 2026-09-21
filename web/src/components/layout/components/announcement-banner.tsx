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
import { Megaphone, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { SiteAnnouncement } from '@/hooks/use-site-announcement'

type AnnouncementBannerProps = {
  announcement: SiteAnnouncement
  onDismiss: () => void
}

/**
 * Full-width notice bar above the page content, the placement enterprise and
 * government portals in this market use for platform-level notices.
 */
export function AnnouncementBanner(props: AnnouncementBannerProps) {
  const { t } = useTranslation()
  return (
    <div className='bg-primary text-primary-foreground relative z-40'>
      <div className='mx-auto flex max-w-7xl items-center gap-3 px-4 py-2.5 md:px-6'>
        <Megaphone className='size-4 shrink-0' aria-hidden='true' />
        <p className='min-w-0 flex-1 truncate text-sm font-medium'>
          {props.announcement.content}
        </p>
        {props.announcement.link && (
          <a
            href={props.announcement.link}
            target='_blank'
            rel='noopener noreferrer'
            className='hidden shrink-0 text-sm underline underline-offset-4 opacity-90 transition-opacity hover:opacity-100 sm:inline'
          >
            {t('Learn more')}
          </a>
        )}
        <button
          type='button'
          onClick={props.onDismiss}
          aria-label={t('Dismiss')}
          className='shrink-0 rounded p-1 opacity-80 transition-opacity hover:opacity-100'
        >
          <X className='size-4' />
        </button>
      </div>
    </div>
  )
}

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
import { useCallback, useEffect, useState } from 'react'

import { useStatus } from '@/hooks/use-status'

const DISMISSED_STORAGE_KEY = 'site-announcement-dismissed'

export interface SiteAnnouncement {
  content: string
  link?: string
  /** Stable identity of the current banner text, used to remember dismissal. */
  fingerprint: string
}

type RawAnnouncement = {
  content?: string
  title?: string
  link?: string
  extra?: string
}

function readDismissed(): string {
  try {
    return localStorage.getItem(DISMISSED_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

/**
 * The single announcement promoted to the site-wide banner: the first
 * configured announcement. Returns null once the visitor dismisses that exact
 * text, and again as soon as an admin publishes different text.
 */
export function useSiteAnnouncement() {
  const { status } = useStatus()
  const [dismissed, setDismissed] = useState<string>(() => readDismissed())

  useEffect(() => {
    setDismissed(readDismissed())
  }, [])

  const enabled = status?.announcements_enabled !== false
  const raw = (status?.announcements as RawAnnouncement[] | undefined)?.[0]
  const content = (raw?.content ?? raw?.title ?? '').trim()
  const fingerprint = content

  const dismiss = useCallback(() => {
    setDismissed(fingerprint)
    try {
      localStorage.setItem(DISMISSED_STORAGE_KEY, fingerprint)
    } catch {
      // A blocked storage just means the banner returns on the next visit.
    }
  }, [fingerprint])

  const visible = enabled && content !== '' && dismissed !== fingerprint
  const announcement: SiteAnnouncement | null = visible
    ? { content, link: raw?.link, fingerprint }
    : null

  return { announcement, dismiss }
}

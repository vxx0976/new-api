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
import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'
import { useSystemConfigStore } from '@/stores/system-config-store'

type BrandLockupProps = {
  /** Sizing for the lockup image; set a height and let the width follow. */
  className?: string
  /** The square logo and site name, shown when no lockup is configured. */
  fallback: React.ReactNode
}

/**
 * The horizontal brand lockup — mark and wordmark as one image — shown in
 * place of the square logo and site name wherever the site brands itself.
 * A wordmark is usually custom-drawn lettering that no font reproduces, so
 * rendering it as text next to the square logo cannot match the brand.
 * Without a configured lockup this renders `fallback` unchanged. The dark
 * variant is optional and falls back to the light one.
 */
export function BrandLockup(props: BrandLockupProps) {
  const systemName = useSystemConfigStore((state) => state.config.systemName)
  const logoWide = useSystemConfigStore((state) => state.config.logoWide)
  const logoWideDark = useSystemConfigStore(
    (state) => state.config.logoWideDark
  )
  const { resolvedTheme } = useTheme()

  const src = resolvedTheme === 'dark' ? logoWideDark || logoWide : logoWide
  if (!src) return props.fallback

  return (
    <img
      src={src}
      alt={systemName}
      className={cn(
        'w-auto max-w-full shrink-0 object-contain',
        props.className
      )}
    />
  )
}

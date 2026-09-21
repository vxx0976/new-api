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
import { Store } from 'lucide-react'

type SupplierAuthHeadingProps = {
  title: string
  children: React.ReactNode
}

/** Shared heading of the supplier auth pages, marked with the supplier icon. */
export function SupplierAuthHeading(props: SupplierAuthHeadingProps) {
  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-center gap-3 sm:justify-start'>
        <span className='bg-primary/10 text-primary flex size-10 items-center justify-center rounded-lg'>
          <Store className='size-5' aria-hidden='true' />
        </span>
        <h2 className='text-2xl font-semibold tracking-tight'>{props.title}</h2>
      </div>
      <p className='text-muted-foreground text-left text-sm sm:text-base'>
        {props.children}
      </p>
    </div>
  )
}

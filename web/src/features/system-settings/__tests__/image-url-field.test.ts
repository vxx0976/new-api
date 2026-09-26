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
import { describe, expect, test } from 'vitest'

import { siteImageUrlSchema } from '../utils/image-url-field'

describe('site image URL field', () => {
  test.each([
    ['a path served by this site', '/platform-logo.png'],
    ['a nested site path', '/assets/brand/logo-wide.svg'],
    ['an absolute https URL', 'https://cdn.example.com/logo.png'],
    ['an empty value, which clears the image', ''],
  ])('accepts %s', (_label, value) => {
    expect(siteImageUrlSchema.safeParse(value).success).toBe(true)
  })

  test.each([
    [
      'a protocol-relative URL, which points at another host',
      '//evil.example.com/logo.png',
    ],
    ['a bare file name with no leading slash', 'logo.png'],
    ['a path containing whitespace', '/my logo.png'],
  ])('rejects %s', (_label, value) => {
    expect(siteImageUrlSchema.safeParse(value).success).toBe(false)
  })
})

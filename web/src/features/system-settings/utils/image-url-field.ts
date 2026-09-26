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
import { z } from 'zod'

// A path served by this site, e.g. /platform-logo.png. Such paths keep
// working when the site moves to a new domain. A second leading slash would
// make the path protocol-relative and point at another host.
const SITE_PATH = /^\/(?!\/)\S+$/

/**
 * An image the site displays, such as its logo: an absolute URL, a path on
 * this site, or empty to clear it.
 */
export const siteImageUrlSchema = z
  .union([z.literal(''), z.string().url(), z.string().regex(SITE_PATH)])
  .optional()

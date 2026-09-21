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
export type ChannelNameRow = {
  /** Channel type id as a string, '' while unselected */
  type: string
  /** One channel name per line */
  names: string
}

type SerializeResult =
  | { ok: true; value: string }
  | { ok: false; errorKey: string }

/** Turns the stored `{ "<type>": ["name"] }` JSON into editable rows. */
export function parseChannelNameRows(value: string): ChannelNameRow[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(value || '{}')
  } catch {
    return []
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return []
  return Object.entries(parsed as Record<string, unknown>).map(
    ([type, names]) => ({
      type,
      names: Array.isArray(names) ? names.map(String).join('\n') : '',
    })
  )
}

/**
 * Builds the option JSON from the rows. Names are trimmed and de-duplicated;
 * a row needs a type and at least one name, and a type may appear only once.
 */
export function serializeChannelNameRows(
  rows: ChannelNameRow[]
): SerializeResult {
  const result: Record<string, string[]> = {}
  for (const row of rows) {
    if (!row.type) {
      return { ok: false, errorKey: 'Select a channel type for every row' }
    }
    if (result[row.type]) {
      return {
        ok: false,
        errorKey: 'Each channel type can only be listed once',
      }
    }
    const names = [
      ...new Set(
        row.names
          .split('\n')
          .map((name) => name.trim())
          .filter(Boolean)
      ),
    ]
    if (names.length === 0) {
      return {
        ok: false,
        errorKey: 'Enter at least one channel name for every type',
      }
    }
    result[row.type] = names
  }
  return { ok: true, value: JSON.stringify(result) }
}

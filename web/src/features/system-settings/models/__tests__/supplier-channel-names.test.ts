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
import { describe, expect, it } from 'vitest'

import {
  parseChannelNameRows,
  serializeChannelNameRows,
} from '../supplier-channel-names'

describe('supplier channel names option', () => {
  it('round-trips the stored map through editable rows', () => {
    const stored = '{"1":["openai official key relay","openai az relay"]}'
    const rows = parseChannelNameRows(stored)

    expect(rows).toEqual([
      { type: '1', names: 'openai official key relay\nopenai az relay' },
    ])
    expect(serializeChannelNameRows(rows)).toEqual({ ok: true, value: stored })
  })

  it('trims names and drops blank and duplicate lines when saving', () => {
    const result = serializeChannelNameRows([
      { type: '14', names: '  claude relay \n\nclaude relay\nclaude max' },
    ])

    expect(result).toEqual({
      ok: true,
      value: '{"14":["claude relay","claude max"]}',
    })
  })

  it('rejects a row without a type, without names, or with a repeated type', () => {
    expect(serializeChannelNameRows([{ type: '', names: 'a' }]).ok).toBe(false)
    expect(serializeChannelNameRows([{ type: '1', names: ' \n' }]).ok).toBe(
      false
    )
    expect(
      serializeChannelNameRows([
        { type: '1', names: 'a' },
        { type: '1', names: 'b' },
      ]).ok
    ).toBe(false)
  })

  it('falls back to no rows when the stored value is not a JSON object', () => {
    expect(parseChannelNameRows('not json')).toEqual([])
    expect(parseChannelNameRows('[]')).toEqual([])
  })
})

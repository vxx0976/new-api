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
import { useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Textarea } from '@/components/ui/textarea'
import { CHANNEL_TYPE_OPTIONS } from '@/features/channels/constants'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  parseChannelNameRows,
  serializeChannelNameRows,
  type ChannelNameRow,
} from './supplier-channel-names'

const OPTION_KEY = 'supplier_setting.channel_names'

type SupplierChannelNamesSectionProps = {
  defaultValue: string
}

export function SupplierChannelNamesSection(
  props: SupplierChannelNamesSectionProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const [rows, setRows] = useState<ChannelNameRow[]>(() =>
    parseChannelNameRows(props.defaultValue)
  )

  useEffect(() => {
    setRows(parseChannelNameRows(props.defaultValue))
  }, [props.defaultValue])

  const typeOptions = useMemo(
    () =>
      CHANNEL_TYPE_OPTIONS.map((option) => ({
        value: String(option.value),
        label: t(option.label),
      })),
    [t]
  )

  const updateRow = (index: number, patch: Partial<ChannelNameRow>) => {
    setRows((current) =>
      current.map((row, i) => (i === index ? { ...row, ...patch } : row))
    )
  }

  const handleSave = async () => {
    const result = serializeChannelNameRows(rows)
    if (!result.ok) {
      toast.error(t(result.errorKey))
      return
    }
    await updateOption.mutateAsync({ key: OPTION_KEY, value: result.value })
    queryClient.invalidateQueries({ queryKey: ['channel_name_options'] })
  }

  return (
    <SettingsSection title={t('Supplier Channel Names')}>
      <SettingsPageFormActions
        onSave={handleSave}
        isSaving={updateOption.isPending}
        saveLabel='Save channel names'
      />
      <div className='space-y-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Suppliers can only create channels of the types listed here, and must pick the channel name from the names defined for that type.'
          )}
        </p>
        {rows.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No channel types are open to suppliers yet.')}
          </p>
        )}
        {rows.map((row, index) => (
          <div
            // Rows have no stable identity besides their position.
            // eslint-disable-next-line react/no-array-index-key
            key={index}
            className='grid gap-3 rounded-lg border p-3 sm:grid-cols-[220px_1fr_auto]'
          >
            <Combobox
              options={typeOptions}
              value={row.type}
              onValueChange={(value) => updateRow(index, { type: value ?? '' })}
              placeholder={t('Select channel type')}
              searchPlaceholder={t('Search channel type...')}
              emptyText={t('No channel type found.')}
            />
            <Textarea
              rows={3}
              value={row.names}
              onChange={(event) =>
                updateRow(index, { names: event.target.value })
              }
              placeholder={t('One channel name per line')}
              aria-label={t('Channel names')}
            />
            <Button
              type='button'
              variant='ghost'
              size='icon'
              aria-label={t('Remove')}
              onClick={() =>
                setRows((current) => current.filter((_, i) => i !== index))
              }
            >
              <Trash2 className='size-4' />
            </Button>
          </div>
        ))}
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            setRows((current) => [...current, { type: '', names: '' }])
          }
        >
          <Plus className='size-4' />
          {t('Add channel type')}
        </Button>
      </div>
    </SettingsSection>
  )
}

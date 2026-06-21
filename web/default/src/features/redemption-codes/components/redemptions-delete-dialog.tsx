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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { deleteRedemption } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import { useRedemptions } from './redemptions-provider'

export function RedemptionsDeleteDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useRedemptions()

  useEffect(() => {
    if (open !== 'delete' || !currentRow) {
      return
    }

    let isMounted = true

    const runDelete = async () => {
      try {
        const result = await deleteRedemption(currentRow.id)
        if (!isMounted) {
          return
        }
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.REDEMPTION_DELETED))
          triggerRefresh()
        }
      } finally {
        if (isMounted) {
          setOpen(null)
        }
      }
    }

    void runDelete()

    return () => {
      isMounted = false
    }
  }, [currentRow, open, setOpen, triggerRefresh])

  return null
}

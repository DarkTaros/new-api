import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { requestHuifuPayment, isApiSuccess } from '../api'

function getJumpURL(data: unknown): string | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  if ('jump_url' in data && typeof data.jump_url === 'string') {
    return data.jump_url
  }
  return null
}

function isSafeHttpURL(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) {
    return false
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function getErrorMessage(message: string | undefined, data: unknown): string {
  if (typeof data === 'string' && data.trim()) {
    return data
  }
  return message || i18next.t('Payment request failed')
}

export function useHuifuPayment() {
  const [processing, setProcessing] = useState(false)

  const processHuifuPayment = useCallback(async (topupAmount: number) => {
    setProcessing(true)
    try {
      const response = await requestHuifuPayment({
        amount: Math.floor(topupAmount),
      })

      if (isApiSuccess(response)) {
        const jumpURL = getJumpURL(response.data)
        if (jumpURL) {
          if (!isSafeHttpURL(jumpURL)) {
            toast.error(i18next.t('Invalid payment redirect URL'))
            return false
          }
          toast.success(i18next.t('Redirecting to payment page...'))
          window.location.href = jumpURL
          return true
        }
      }

      toast.error(getErrorMessage(response.message, response.data))
      return false
    } catch (_error) {
      toast.error(i18next.t('Payment request failed'))
      return false
    } finally {
      setProcessing(false)
    }
  }, [])

  return { processing, processHuifuPayment }
}

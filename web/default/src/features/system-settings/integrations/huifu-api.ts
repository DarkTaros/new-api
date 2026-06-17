import { api } from '@/lib/api'

interface BackendBody<T> {
  success?: boolean
  message?: string
  data?: T | string
}

export type HuifuSaveResponse = BackendBody<{
  sys_id: string
  product_id: string
  merchant_id: string
  project_id: string
  skill_source: string
  notify_url: string
}>

export async function saveHuifuConfig(params: {
  sysID: string
  productID: string
  merchantID: string
  projectID: string
  skillSource: string
  rsaPrivateKey: string
  rsaPublicKey: string
  notifyURL: string
}): Promise<HuifuSaveResponse> {
  const res = await api.post<HuifuSaveResponse>('/api/option/huifu/save', {
    sys_id: params.sysID,
    product_id: params.productID,
    merchant_id: params.merchantID,
    project_id: params.projectID,
    skill_source: params.skillSource,
    rsa_private_key: params.rsaPrivateKey,
    rsa_public_key: params.rsaPublicKey,
    notify_url: params.notifyURL,
  })
  return res.data
}

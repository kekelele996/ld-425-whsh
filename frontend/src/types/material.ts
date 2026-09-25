import type { PurchaseStatus } from './enums'

export interface MaterialItem {
  id: number
  project_id: number
  name: string
  category: string
  spec: string
  brand: string
  quantity: number
  unit: string
  unit_price: number
  total_price: number
  purchase_status: PurchaseStatus
  supplier: string
  space: string
  // 累计已安装量：验收通过的节点用料登记合计。
  installed_quantity: number
  // 剩余量：采购量 - 已安装量（待确认记录不参与计算）。
  remaining_quantity: number
  created_at: string
  updated_at: string
}

export interface CreateMaterialRequest {
  project_id: number
  name: string
  category: string
  spec?: string
  brand?: string
  quantity: number
  unit: string
  unit_price?: number
  supplier?: string
  space?: string
}

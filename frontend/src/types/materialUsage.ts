import type { MaterialUsageStatus } from './enums'

export interface MaterialUsage {
  id: number
  project_id: number
  node_id: number
  material_id: number
  material_name: string
  quantity: number
  unit: string
  status: MaterialUsageStatus
  remark: string
  created_at: string
  updated_at: string
}

export interface RegisterMaterialUsageRequest {
  node_id: number
  material_id: number
  quantity: number
  remark?: string
}

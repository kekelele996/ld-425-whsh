import type { ConstructionStatus, AcceptanceStatus, UsageStatus } from './enums'

export interface MaterialUsage {
  id: number
  project_id: number
  node_id: number
  material_id: number
  material_name: string
  material_unit: string
  quantity: number
  status: UsageStatus
  note: string
  created_at: string
  updated_at: string
}

export interface RegisterMaterialUsageRequest {
  material_id: number
  quantity: number
  note?: string
  client_key?: string
}

export interface ConstructionNode {
  id: number
  project_id: number
  name: string
  planned_start_date: string | null
  planned_end_date: string | null
  actual_start_date: string | null
  actual_end_date: string | null
  status: ConstructionStatus
  acceptance_status: AcceptanceStatus
  acceptance_photos: string[]
  acceptance_note: string
  // 每个节点的用料登记明细，施工页展开查看。
  usages: MaterialUsage[]
  created_at: string
  updated_at: string
}

export interface AcceptConstructionRequest {
  accepted: boolean
  photos?: string[]
  note?: string
}

export interface CreateConstructionRequest {
  project_id: number
  name: string
  planned_start_date?: string
  planned_end_date?: string
}

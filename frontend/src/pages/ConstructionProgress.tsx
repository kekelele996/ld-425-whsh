import { useEffect, useMemo, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography, message, Upload } from 'antd'
import { CheckOutlined, PlayCircleOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons'
import StatusBadge from '@/components/common/StatusBadge'
import Timeline from '@/components/common/Timeline'
import StepIndicator from '@/components/common/StepIndicator'
import EmptyState from '@/components/common/EmptyState'
import { useConstructionStore } from '@/stores/constructionStore'
import { useProjectStore } from '@/stores/projectStore'
import { useMaterialStore } from '@/stores/materialStore'
import { useAuthStore } from '@/stores/authStore'
import {
  acceptConstruction,
  createConstruction,
  deleteMaterialUsage,
  registerNodeUsage,
  updateConstructionStatus,
} from '@/api/construction'
import { uploadFile } from '@/utils/upload'
import { extractErrorMessage } from '@/utils/request'
import { ConstructionStatus, Role, ConstructionName, PurchaseStatus, UsageStatus, AcceptanceStatus } from '@/types/enums'
import type { ConstructionNode, MaterialItem, MaterialUsage } from '@/types'

const usageStatusMeta: Record<string, { label: string; color: string }> = {
  [UsageStatus.Registered]: { label: '已登记，待验收', color: 'gold' },
  [UsageStatus.Confirmed]: { label: '已计入安装', color: 'green' },
  [UsageStatus.PendingConfirm]: { label: '待确认', color: 'red' },
}

function newClientKey() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `usage-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export default function ConstructionProgress() {
  const { nodes, fetchNodes } = useConstructionStore()
  const { projects, fetchProjects } = useProjectStore()
  const { materials, fetchMaterials } = useMaterialStore()
  const user = useAuthStore((state) => state.user)
  const [projectId, setProjectId] = useState<number>()
  const [acceptTarget, setAcceptTarget] = useState<ConstructionNode | null>(null)
  const [usageTarget, setUsageTarget] = useState<ConstructionNode | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [photoUrls, setPhotoUrls] = useState<string[]>([])
  const [form] = Form.useForm()
  const [acceptForm] = Form.useForm()
  const [usageForm] = Form.useForm()
  // 一次登记会话一个幂等键，重复提交只保留第一次结果。
  const [clientKey, setClientKey] = useState(newClientKey())

  useEffect(() => {
    fetchProjects()
  }, [fetchProjects])

  useEffect(() => {
    fetchNodes(projectId)
    fetchMaterials(projectId)
  }, [fetchNodes, fetchMaterials, projectId])

  const filtered = useMemo(() => {
    if (!projectId) return nodes
    return nodes.filter((n) => n.project_id === projectId)
  }, [nodes, projectId])

  const canOperate = user?.role === Role.Admin || user?.role === Role.Contractor || user?.role === Role.ProjectManager

  // 只有已到货的材料才能登记安装用料。
  const deliveredMaterials = useMemo(
    () => materials.filter((m) => m.purchase_status === PurchaseStatus.Delivered || m.purchase_status === PurchaseStatus.Installed),
    [materials],
  )

  const startNode = async (node: ConstructionNode) => {
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.InProgress)
      message.success('节点已开工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const completeNode = async (node: ConstructionNode) => {
    // 节点完工前应登记本次用料；无材料工序（如拆改）可确认后继续。
    if (node.usages.length === 0) {
      const ok = window.confirm('该节点尚未登记任何用料，确定直接完工吗？')
      if (!ok) return
    }
    try {
      await updateConstructionStatus(node.id, ConstructionStatus.Completed)
      message.success('节点已完工')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onAccept = async () => {
    const values = await acceptForm.validateFields()
    if (!acceptTarget) return
    try {
      await acceptConstruction(acceptTarget.id, { ...values, photos: photoUrls })
      message.success(values.accepted ? '验收通过，用料已计入安装量' : '验收不通过，用料已转为待确认')
      setAcceptTarget(null)
      setPhotoUrls([])
      acceptForm.resetFields()
      await fetchNodes(projectId)
      await fetchMaterials(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onRegisterUsage = async () => {
    const values = await usageForm.validateFields()
    if (!usageTarget) return
    try {
      await registerNodeUsage(usageTarget.id, { ...values, client_key: clientKey })
      message.success('用料已登记')
      setUsageTarget(null)
      usageForm.resetFields()
      setClientKey(newClientKey())
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const removeUsage = async (usage: MaterialUsage) => {
    try {
      await deleteMaterialUsage(usage.id)
      message.success('已删除')
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  const onCreate = async () => {
    const values = await form.validateFields()
    try {
      await createConstruction({ ...values, project_id: projectId! })
      message.success('创建成功')
      setCreateOpen(false)
      form.resetFields()
      await fetchNodes(projectId)
    } catch (error) {
      message.error(extractErrorMessage(error))
    }
  }

  // 节点是否可登记用料：施工中，或完工但验收不通过（返工补登）。
  const canRegisterUsage = (node: ConstructionNode) =>
    node.status === ConstructionStatus.InProgress ||
    (node.status === ConstructionStatus.Completed && node.acceptance_status === AcceptanceStatus.Failed)

  const renderUsageTable = (node: ConstructionNode) => {
    if (node.usages.length === 0) {
      return <EmptyState description="暂无用料登记，请在节点完工前登记本次用料" />
    }
    return (
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={node.usages}
        columns={[
          { title: '材料', dataIndex: 'material_name' },
          {
            title: '本次用料',
            render: (_, record) => `${record.quantity} ${record.material_unit}`,
          },
          {
            title: '状态',
            dataIndex: 'status',
            render: (v: string) => {
              const meta = usageStatusMeta[v]
              return <Tag color={meta?.color ?? 'default'}>{meta?.label ?? v}</Tag>
            },
          },
          { title: '备注', dataIndex: 'note' },
          ...(canOperate
            ? [
                {
                  title: '操作',
                  render: (_: unknown, record: MaterialUsage) =>
                    record.status !== UsageStatus.Confirmed ? (
                      <Button size="small" danger onClick={() => removeUsage(record)}>
                        删除
                      </Button>
                    ) : null,
                },
              ]
            : []),
        ]}
      />
    )
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={3} style={{ margin: 0 }}>施工进度</Typography.Title>
        <Select
          style={{ width: 240 }}
          placeholder="选择项目"
          allowClear
          value={projectId}
          onChange={setProjectId}
          options={projects.map((p) => ({ label: p.name, value: p.id }))}
        />
        {user?.role === Role.Admin || user?.role === Role.ProjectManager ? (
          <Button type="primary" disabled={!projectId} onClick={() => setCreateOpen(true)}>新增节点</Button>
        ) : null}
      </Space>

      <StepIndicator
        current={filtered.filter((n) => n.status === ConstructionStatus.Completed).length}
        items={filtered.map((n) => n.name)}
      />

      <Card style={{ marginTop: 16 }}>
        <Timeline
          items={filtered.map((node) => ({
            title: node.name,
            status: node.status,
            time: `${node.planned_start_date || '-'} ~ ${node.planned_end_date || '-'}`,
            description: `验收状态：${node.acceptance_status}${node.acceptance_note ? ' · ' + node.acceptance_note : ''}`,
          }))}
        />
      </Card>

      <Card title="节点用料与验收（点击行首展开查看每个节点的用料）" style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          dataSource={filtered}
          pagination={false}
          expandable={{ expandedRowRender: renderUsageTable }}
          columns={[
            { title: '节点名称', dataIndex: 'name' },
            { title: '状态', dataIndex: 'status', render: (v) => <StatusBadge status={v} /> },
            { title: '验收状态', dataIndex: 'acceptance_status', render: (v) => <StatusBadge status={v} /> },
            {
              title: '用料登记',
              render: (_, record) => {
                const confirmed = record.usages.filter((u) => u.status === UsageStatus.Confirmed).length
                const pending = record.usages.filter((u) => u.status !== UsageStatus.Confirmed).length
                return (
                  <Space size={4}>
                    <Tag>{record.usages.length} 条</Tag>
                    {confirmed > 0 ? <Tag color="green">已计入 {confirmed}</Tag> : null}
                    {pending > 0 ? <Tag color="gold">未计入 {pending}</Tag> : null}
                  </Space>
                )
              },
            },
            { title: '计划开始', dataIndex: 'planned_start_date' },
            { title: '计划结束', dataIndex: 'planned_end_date' },
            {
              title: '操作',
              render: (_, record) =>
                canOperate ? (
                  <Space wrap>
                    {record.status === ConstructionStatus.Pending ? (
                      <Button size="small" icon={<PlayCircleOutlined />} onClick={() => startNode(record)}>开工</Button>
                    ) : null}
                    {record.status === ConstructionStatus.InProgress ? (
                      <>
                        <Button size="small" icon={<PlusOutlined />} onClick={() => setUsageTarget(record)}>登记用料</Button>
                        <Button size="small" type="primary" onClick={() => completeNode(record)}>完工</Button>
                      </>
                    ) : null}
                    {record.status === ConstructionStatus.Completed && record.acceptance_status !== AcceptanceStatus.Passed ? (
                      <>
                        {record.acceptance_status === AcceptanceStatus.Failed ? (
                          <Button size="small" icon={<PlusOutlined />} onClick={() => setUsageTarget(record)}>返工补登用料</Button>
                        ) : null}
                        <Button size="small" icon={<CheckOutlined />} onClick={() => setAcceptTarget(record)}>
                          {record.acceptance_status === AcceptanceStatus.Failed ? '重新验收' : '验收'}
                        </Button>
                      </>
                    ) : null}
                  </Space>
                ) : null,
            },
          ]}
        />
      </Card>

      <Modal title="新增施工节点" open={createOpen} onOk={onCreate} onCancel={() => setCreateOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="节点名称" rules={[{ required: true }]}>
            <Select options={ConstructionName.map((n) => ({ label: n, value: n }))} />
          </Form.Item>
          <Form.Item name="planned_start_date" label="计划开始日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
          <Form.Item name="planned_end_date" label="计划结束日期"><Input placeholder="YYYY-MM-DD" /></Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`登记用料 · ${usageTarget?.name ?? ''}`}
        open={!!usageTarget}
        onOk={onRegisterUsage}
        onCancel={() => {
          setUsageTarget(null)
          usageForm.resetFields()
          setClientKey(newClientKey())
        }}
      >
        <Typography.Paragraph type="secondary">
          请在节点完工前登记本次用料；验收通过后自动计入材料已安装量，重复提交只保留第一次结果。
        </Typography.Paragraph>
        <Form form={usageForm} layout="vertical" initialValues={{ quantity: 1 }}>
          <Form.Item name="material_id" label="材料（仅可选择已到货材料）" rules={[{ required: true, message: '请选择材料' }]}>
            <Select
              placeholder="选择材料"
              options={deliveredMaterials.map((m: MaterialItem) => ({
                label: `${m.name}（剩余 ${m.remaining_quantity} ${m.unit}）`,
                value: m.id,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="quantity"
            label="本次用料数量"
            rules={[
              { required: true, message: '请输入数量' },
              {
                validator: (_, value) => {
                  const materialId = usageForm.getFieldValue('material_id')
                  const material = deliveredMaterials.find((m) => m.id === materialId)
                  if (material && value > material.remaining_quantity) {
                    return Promise.reject(new Error(`不能超过剩余量 ${material.remaining_quantity} ${material.unit}`))
                  }
                  return Promise.resolve()
                },
              },
            ]}
          >
            <InputNumber min={0.01} step={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="note" label="用途说明">
            <Input placeholder="如：客厅地面铺贴" maxLength={200} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`施工验收 · ${acceptTarget?.name ?? ''}`}
        open={!!acceptTarget}
        onOk={onAccept}
        onCancel={() => {
          setAcceptTarget(null)
          setPhotoUrls([])
          acceptForm.resetFields()
        }}
      >
        {acceptTarget ? (
          <Typography.Paragraph type="secondary">
            本节点已登记 {acceptTarget.usages.length} 条用料。
            验收通过后计入材料已安装量（累计不超过采购量）；一次验收不通过将转为待确认，不参与余量计算。
          </Typography.Paragraph>
        ) : null}
        {acceptTarget && acceptTarget.usages.length > 0 ? (
          <Table
            rowKey="id"
            size="small"
            pagination={false}
            style={{ marginBottom: 16 }}
            dataSource={acceptTarget.usages}
            columns={[
              { title: '材料', dataIndex: 'material_name' },
              { render: (_, record) => `${record.quantity} ${record.material_unit}` },
            ]}
          />
        ) : null}
        <Form form={acceptForm} layout="vertical" initialValues={{ accepted: true }}>
          <Form.Item name="accepted" label="验收结论" rules={[{ required: true }]}>
            <Select options={[{ label: '验收通过', value: true }, { label: '验收不通过', value: false }]} />
          </Form.Item>
          <Form.Item name="note" label="验收说明"><Input.TextArea rows={3} /></Form.Item>
          <Form.Item label="验收照片">
            <Upload
              beforeUpload={async (file) => {
                try {
                  const url = await uploadFile(file)
                  setPhotoUrls((prev) => [...prev, url])
                } catch (error) {
                  message.error(extractErrorMessage(error))
                }
                return false
              }}
            >
              <Button icon={<UploadOutlined />}>上传照片</Button>
            </Upload>
            {photoUrls.map((url) => (
              <div key={url} style={{ marginTop: 8 }}>{url}</div>
            ))}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

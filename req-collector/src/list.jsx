const { useState, useEffect } = React;
const { Table, Tag, Select, Button, Card, Typography, Message, Space, Empty, Modal, Form, Input, Popconfirm, Alert } = window.arco;
const FormItem = Form.Item;
const TextArea = Input.TextArea;

const BASE_DEPARTMENTS = window.APP_CONFIG.departments;
const REQ_TYPES = window.APP_CONFIG.req_types;
const URGENCIES = window.APP_CONFIG.urgencies;
const BILLS = window.APP_CONFIG.bills;
const FEASIBILITIES = window.APP_CONFIG.feasibilities;

const URGENCY_COLOR = { "紧急": "red", "高": "orange", "中": "gray", "低": "gray" };
const FEAS_COLOR = { "待评估": "gray", "能实现": "green", "需二开": "gold", "不能实现": "red" };

function App() {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [me, setMe] = useState({ logged: false, is_admin: false, userid: null });
  const [authState, setAuthState] = useState(window.WBAuth.state);
  const [editing, setEditing] = useState(null);
  const [editForm] = Form.useForm();

  useEffect(() => {
    return window.WBAuth.subscribe(function (s) { setAuthState(Object.assign({}, s)); });
  }, []);

  const load = async () => {
    setLoading(true);
    try {
      const meData = await window.WBAuth.ensureLogin();
      setMe({
        logged: !!meData.logged, is_admin: !!meData.is_admin,
        name: meData.name, department: meData.department, userid: meData.userid || null,
      });
      const data = await window.WBAuth.api("/api/requirements");
      setRows(data.rows || []);
    } catch (e) { Message.error("加载失败：" + window.WBAuth.errText(e)); }
    setLoading(false);
  };
  useEffect(() => { load(); }, []);

  const post = (url, body) => window.WBAuth.api(url, { method: "POST", body: JSON.stringify(body) });

  const setFeasibility = async (id, feasibility) => {
    const data = await post("/api/feasibility", { id, feasibility });
    if (data.ok) { Message.success("已更新"); load(); }
    else Message.error(data.error || "更新失败");
  };

  const removeRow = async (id) => {
    const data = await post("/api/admin/requirement/delete", { id });
    if (data.ok) { Message.success("已删除"); load(); }
    else Message.error(data.error || "删除失败");
  };

  const openEdit = (record) => {
    setEditing(record);
    editForm.setFieldsValue({
      department: record.department, requester: record.requester,
      description: record.description, scenario: record.scenario,
      bill_name: record.bill_name, req_type: record.req_type,
      urgency: record.urgency, feasibility: record.feasibility,
      remark: record.remark,
    });
  };

  const saveEdit = async (values) => {
    const data = await post("/api/admin/requirement/update", Object.assign({ id: editing.id }, values));
    if (data.ok) { Message.success("已保存"); setEditing(null); load(); }
    else Message.error(data.error || "保存失败");
  };

  // 自己提交的记录：可编辑可删除
  const isMine = (r) => !!(me.userid && r.username === me.userid);
  const canManage = (r) => me.is_admin || isMine(r);

  const feasibilityCol = me.is_admin
    ? { title: "能否实现", dataIndex: "feasibility", width: 130,
        render: (v, r) => (
          <Select size="small" value={v} style={{ width: 108 }}
                  onChange={val => setFeasibility(r.id, val)}>
            {FEASIBILITIES.map(f => <Select.Option key={f} value={f}>{f}</Select.Option>)}
          </Select>) }
    : { title: "能否实现", dataIndex: "feasibility", width: 100,
        render: v => <Tag color={FEAS_COLOR[v] || "gray"}>{v}</Tag> };

  const deptOptions = me.department && BASE_DEPARTMENTS.indexOf(me.department) === -1
    ? BASE_DEPARTMENTS.concat([me.department]) : BASE_DEPARTMENTS;

  const columns = [
    { title: "#", dataIndex: "id", width: 56 },
    { title: "部门", dataIndex: "department", width: 84 },
    { title: "需求人", dataIndex: "requester", width: 100,
      render: (v, r) => <span>{v} {isMine(r) ? <Tag size="small" color="arcoblue">我</Tag> : null}</span> },
    { title: "需求描述", dataIndex: "description", width: 260 },
    { title: "应用场景", dataIndex: "scenario", width: 200, render: v => v || "-" },
    { title: "单据和类型", dataIndex: "bill_name", width: 180,
      render: (v, r) => <span>{v} <Tag size="small" color="arcoblue">{r.req_type}</Tag></span> },
    { title: "需求时间", dataIndex: "created_at", width: 140 },
    { title: "紧急程度", dataIndex: "urgency", width: 96,
      render: v => <Tag color={URGENCY_COLOR[v] || "gray"}>{v}</Tag> },
    feasibilityCol,
    { title: "备注", dataIndex: "remark", width: 140, render: v => v || "-" },
  ];

  if (me.is_admin || me.logged) {
    columns.push({
      title: "操作", dataIndex: "_op", width: 130, fixed: "right",
      render: (v, r) => canManage(r) ? (
        <Space>
          <Button size="mini" onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="确认删除该条需求？删除后不可恢复" onOk={() => removeRow(r.id)}>
            <Button size="mini" status="danger" type="text">删除</Button>
          </Popconfirm>
        </Space>)
        : <Typography.Text type="secondary">仅本人可操作</Typography.Text> },
    );
  }

  const renderAuthBanner = () => {
    const st = authState.status;
    if (st === "fail") {
      return (
        <Alert type="error" style={{ marginBottom: 12 }}
          title="钉钉免登失败，当前为游客身份（无法编辑自己的需求）"
          content={
            <div>
              <div>原因：{authState.message || "未知"}</div>
              {authState.tried && authState.tried.length > 0 &&
                <div style={{ marginTop: 4, color: "#86909c", fontSize: 12 }}>尝试记录：{authState.tried.join("；")}</div>}
              <Button size="small" type="primary" style={{ marginTop: 8 }}
                      onClick={() => load()}>重新免登</Button>
            </div>
          } />
      );
    }
    if (st === "ok" && me.logged && !me.is_admin) {
      return (
        <Alert type="success" style={{ marginBottom: 12 }}
          content={"当前身份：" + me.name + (me.department ? "（" + me.department + "）" : "") + "，带「我」标签的记录可自行编辑或删除"} />
      );
    }
    if (st === "outside") {
      return (
        <Alert type="normal" style={{ marginBottom: 12 }}
          content="当前在普通浏览器中打开（非钉钉客户端），可查看全部需求；在钉钉里打开可用本人身份编辑自己的需求" />
      );
    }
    return null;
  };

  return (
    <div className="wrap">
      <div className="topbar">
        <Typography.Title heading={4} style={{ margin: 0 }}>
          需求列表（{rows.length} 条）
          {me.is_admin ? <Tag color="green" style={{ marginLeft: 8 }}>管理模式</Tag> : null}
        </Typography.Title>
        <Space>
          <Button onClick={load} loading={loading}>刷新</Button>
          <Button type="primary" href="/">+ 填写新需求</Button>
          {me.is_admin ? <Button href="/admin">管理面板</Button> : null}
        </Space>
      </div>

      {renderAuthBanner()}

      <Card bodyStyle={{ padding: 8 }}>
        <Table columns={columns} data={rows} rowKey="id" loading={loading}
               pagination={{ pageSize: 20 }} scroll={{ x: 1500 }}
               noDataElement={<Empty description="暂无需求数据" />} />
      </Card>

      <Modal title={"编辑需求 #" + (editing ? editing.id : "")} visible={!!editing}
             onCancel={() => setEditing(null)} onOk={() => editForm.submit()} okText="保存">
        <Form form={editForm} layout="vertical" onSubmit={saveEdit}>
          <FormItem label="部门" field="department" rules={[{ required: true }]}>
            <Select options={deptOptions} />
          </FormItem>
          <FormItem label="需求人" field="requester" rules={[{ required: true }]}>
            <Input maxLength={50} />
          </FormItem>
          <FormItem label="需求描述" field="description" rules={[{ required: true }]}>
            <TextArea autoSize={{ minRows: 2 }} />
          </FormItem>
          <FormItem label="应用场景" field="scenario">
            <TextArea autoSize={{ minRows: 2 }} />
          </FormItem>
          <FormItem label="单据名称" field="bill_name" rules={[{ required: true }]}>
            <Input maxLength={100} />
          </FormItem>
          <FormItem label="需求类型" field="req_type" rules={[{ required: true }]}>
            <Select options={REQ_TYPES} />
          </FormItem>
          <FormItem label="紧急程度" field="urgency">
            <Select options={URGENCIES} />
          </FormItem>
          {me.is_admin ? (
            <FormItem label="能否实现" field="feasibility">
              <Select options={FEASIBILITIES} />
            </FormItem>
          ) : null}
          <FormItem label="备注" field="remark">
            <Input maxLength={500} />
          </FormItem>
        </Form>
      </Modal>
    </div>
  );
}
ReactDOM.createRoot(document.getElementById("root")).render(<App />);

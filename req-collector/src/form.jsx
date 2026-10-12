const { useState, useEffect } = React;
const { Form, Input, Select, Radio, Button, Card, Typography, Message, AutoComplete, Grid, Alert, Space, Tag } = window.arco;
const FormItem = Form.Item;
const TextArea = Input.TextArea;

const BASE_DEPARTMENTS = window.APP_CONFIG.departments;
const REQ_TYPES = window.APP_CONFIG.req_types;
const URGENCIES = window.APP_CONFIG.urgencies;
const BILLS = window.APP_CONFIG.bills;

const STATUS_TEXT = {
  unknown: "正在检查登录状态…",
  logging: "正在钉钉免登…",
  ok: "",
  fail: "钉钉免登失败",
  outside: ""
};

function App() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [me, setMe] = useState(null);                 // { name, department }
  const [authState, setAuthState] = useState(window.WBAuth.state);
  const [deptOptions, setDeptOptions] = useState(BASE_DEPARTMENTS);
  const [showDiag, setShowDiag] = useState(false);

  // 订阅鉴权状态变化
  useEffect(() => {
    return window.WBAuth.subscribe(function (s) { setAuthState(Object.assign({}, s)); });
  }, []);

  // 部门下拉：钉钉带回的部门若不在列表里，动态补进去
  useEffect(() => {
    if (me && me.department && BASE_DEPARTMENTS.indexOf(me.department) === -1) {
      setDeptOptions(BASE_DEPARTMENTS.concat([me.department]));
    }
  }, [me]);

  // 登录 / 免登
  useEffect(() => {
    window.WBAuth.ensureLogin().then(function (m) {
      if (m && m.logged) setMe({ name: m.name, department: m.department });
    });
  }, []);

  // 免登成功后回填部门 / 姓名
  useEffect(() => {
    if (!me) return;
    var patch = {};
    if (me.department) patch.department = me.department;
    if (me.name) patch.requester = me.name;
    if (Object.keys(patch).length) form.setFieldsValue(patch);
  }, [me]);

  const onSubmit = async (values) => {
    setLoading(true);
    try {
      const data = await window.WBAuth.api("/api/submit", {
        method: "POST",
        body: JSON.stringify(values),
      });
      if (data.ok) {
        Message.success("提交成功，感谢反馈！可继续填写下一条需求");
        form.resetFields();
        form.setFieldsValue({ urgency: "中", department: me && me.department ? me.department : BASE_DEPARTMENTS[0], requester: me ? me.name : "" });
      } else {
        Message.error("提交失败：" + (data.error || "未知错误"));
      }
    } catch (e) { Message.error("网络异常：" + e.message); }
    setLoading(false);
  };

  // 免登状态提示条
  const renderAuthBanner = () => {
    const st = authState.status;
    if (st === "ok" && me) {
      return (
        <Alert type="success" style={{ marginBottom: 12 }}
          content={<span>已完成钉钉免登：<b>{me.name}</b>{me.department ? "（" + me.department + "）" : ""}，提交时自动以此身份记录</span>} />
      );
    }
    if (st === "fail") {
      return (
        <Alert type="error" style={{ marginBottom: 12 }}
          title="钉钉免登失败，可先手动填写姓名提交"
          content={
            <div>
              <div>原因：{authState.message || "未知"}</div>
              {authState.tried && authState.tried.length > 0 && (
                <div style={{ marginTop: 4, color: "#86909c", fontSize: 12 }}>
                  尝试记录：{authState.tried.join("；")}
                </div>
              )}
              <Space style={{ marginTop: 8 }}>
                <Button size="small" type="primary"
                        onClick={() => { window.WBAuth.ensureLogin(true).then(function (m) { if (m && m.logged) setMe({ name: m.name, department: m.department }); }); }}>
                  重新免登
                </Button>
                <Button size="small" onClick={() => setShowDiag(!showDiag)}>查看诊断信息</Button>
              </Space>
              {showDiag && (
                <pre style={{ marginTop: 8, fontSize: 12, whiteSpace: "pre-wrap", background: "#f7f8fa", padding: 8, borderRadius: 4 }}>
{"环境： " + (authState.env || window.WBAuth.envInfo()) + "\n" +
 "UA： " + navigator.userAgent + "\n" +
 "URL： " + location.href + "\n" +
 "旧版 session：" + (window.WBAuth.getToken() ? "有 token" : "无 token")}
                </pre>
              )}
            </div>
          } />
      );
    }
    if (st === "logging") {
      return <Alert type="info" style={{ marginBottom: 12 }} content={STATUS_TEXT[st]} />;
    }
    if (st === "outside") {
      return (
        <Alert type="normal" style={{ marginBottom: 12 }}
          content="当前在普通浏览器中打开（非钉钉客户端），请手动选择部门并填写姓名" />
      );
    }
    return null;
  };

  return (
    <div className="wrap">
      <div className="topbar">
        <div>
          <Typography.Title heading={4} style={{ marginBottom: 2 }}>
            {(window.APP_CONFIG.app_title || "需求收集") + "表"}
          </Typography.Title>
          <Typography.Text type="secondary">填写你的需求，提交后由管理员统一评审并反馈处理结论</Typography.Text>
        </div>
        <Space>
          <Button href="/list">需求列表</Button>
          <Button href="/admin">管理面板</Button>
        </Space>
      </div>

      {renderAuthBanner()}

      <Card>
        <Form form={form} layout="vertical" onSubmit={onSubmit}
              initialValues={{ urgency: "中", department: BASE_DEPARTMENTS[0] || "", req_type: REQ_TYPES[0] || "" }}>
          <Grid.Row gutter={16}>
            <Grid.Col span={12}>
              <FormItem label={me && me.department ? "部门（钉钉自动识别）" : "部门"}
                        field="department" rules={[{ required: true, message: "请选择部门" }]}>
                {me && me.department
                  ? <Input value={me.department} disabled />
                  : <Select options={deptOptions} placeholder="请选择部门" />}
              </FormItem>
            </Grid.Col>
            <Grid.Col span={12}>
              <FormItem label={me ? "需求人（钉钉免登）" : "需求人"}
                        field="requester"
                        rules={me ? [] : [{ required: true, message: "请填写姓名" }]}>
                {me
                  ? <Input value={me.name} disabled />
                  : <Input placeholder="您的姓名" maxLength={50} />}
              </FormItem>
            </Grid.Col>
          </Grid.Row>
          <FormItem label="需求描述" field="description" rules={[{ required: true, message: "请填写需求描述" }]}>
            <TextArea placeholder="具体描述要做什么，写清期望的业务效果" autoSize={{ minRows: 3 }} />
          </FormItem>
          <FormItem label="需求应用场景" field="scenario">
            <TextArea placeholder="在什么业务场景下使用、解决什么痛点" autoSize={{ minRows: 2 }} />
          </FormItem>
          <Grid.Row gutter={16}>
            <Grid.Col span={12}>
              <FormItem label="单据名称" field="bill_name" rules={[{ required: true, message: "请填写单据名称" }]}>
                <AutoComplete data={BILLS} placeholder="如：采购订单 / 销售出库单" />
              </FormItem>
            </Grid.Col>
            <Grid.Col span={12}>
              <FormItem label="需求类型" field="req_type" rules={[{ required: true }]}>
                <Select options={REQ_TYPES} />
              </FormItem>
            </Grid.Col>
          </Grid.Row>
          <FormItem label="紧急程度" field="urgency">
            <Radio.Group type="button" options={URGENCIES} />
          </FormItem>
          <FormItem label="备注" field="remark">
            <Input placeholder="关联单据、依赖条件等补充信息（选填）" maxLength={500} />
          </FormItem>
          <Button type="primary" htmlType="submit" long size="large" loading={loading}>提交需求</Button>
        </Form>
      </Card>
    </div>
  );
}
ReactDOM.createRoot(document.getElementById("root")).render(<App />);

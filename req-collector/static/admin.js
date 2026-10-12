/*!
 * 本文件由 src/admin.jsx 自动编译生成（node scripts/build_frontend.js）。
 * 请勿直接修改本文件，改源码后重新编译，并更新 handlers.go 中的 staticVer 以刷新钉钉缓存。
 */
const {
  useState,
  useEffect,
  useRef
} = React;
const {
  Tabs,
  Table,
  Tag,
  Select,
  Button,
  Card,
  Typography,
  Message,
  Space,
  Empty,
  Modal,
  Form,
  Input,
  Popconfirm,
  Alert
} = window.arco;
const TabPane = Tabs.TabPane;
const FormItem = Form.Item;
const TextArea = Input.TextArea;
const DEPARTMENTS = window.APP_CONFIG.departments;
const REQ_TYPES = window.APP_CONFIG.req_types;
const URGENCIES = window.APP_CONFIG.urgencies;
const FEASIBILITIES = window.APP_CONFIG.feasibilities;
const URGENCY_COLOR = {
  "紧急": "red",
  "高": "orange",
  "中": "gray",
  "低": "gray"
};
function post(url, body) {
  return window.WBAuth.api(url, {
    method: "POST",
    body: JSON.stringify(body)
  });
}

// ---------------- 需求管理 ----------------
function RequirementManager() {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState(null);
  const [editForm] = Form.useForm();
  const load = async () => {
    setLoading(true);
    const data = await window.WBAuth.api("/api/requirements");
    setRows(data.rows || []);
    setLoading(false);
  };
  useEffect(() => {
    load();
  }, []);
  const removeRow = async id => {
    const data = await post("/api/admin/requirement/delete", {
      id
    });
    if (data.ok) {
      Message.success("已删除");
      load();
    } else Message.error(data.error || "删除失败");
  };
  const openEdit = record => {
    setEditing(record);
    editForm.setFieldsValue({
      department: record.department,
      requester: record.requester,
      description: record.description,
      scenario: record.scenario,
      bill_name: record.bill_name,
      req_type: record.req_type,
      urgency: record.urgency,
      feasibility: record.feasibility,
      remark: record.remark
    });
  };
  const saveEdit = async values => {
    const data = await post("/api/admin/requirement/update", {
      id: editing.id,
      ...values
    });
    if (data.ok) {
      Message.success("已保存");
      setEditing(null);
      load();
    } else Message.error(data.error || "保存失败");
  };
  const columns = [{
    title: "#",
    dataIndex: "id",
    width: 56
  }, {
    title: "部门",
    dataIndex: "department",
    width: 84
  }, {
    title: "需求人",
    dataIndex: "requester",
    width: 84
  }, {
    title: "需求描述",
    dataIndex: "description",
    width: 240
  }, {
    title: "单据和类型",
    dataIndex: "bill_name",
    width: 170,
    render: (v, r) => /*#__PURE__*/React.createElement("span", null, v, " ", /*#__PURE__*/React.createElement(Tag, {
      size: "small",
      color: "arcoblue"
    }, r.req_type))
  }, {
    title: "需求时间",
    dataIndex: "created_at",
    width: 140
  }, {
    title: "紧急程度",
    dataIndex: "urgency",
    width: 90,
    render: v => /*#__PURE__*/React.createElement(Tag, {
      color: URGENCY_COLOR[v] || "gray"
    }, v)
  }, {
    title: "能否实现",
    dataIndex: "feasibility",
    width: 100
  }, {
    title: "操作",
    dataIndex: "_op",
    width: 130,
    fixed: "right",
    render: (v, r) => /*#__PURE__*/React.createElement(Space, null, /*#__PURE__*/React.createElement(Button, {
      size: "mini",
      onClick: () => openEdit(r)
    }, "编辑"), /*#__PURE__*/React.createElement(Popconfirm, {
      title: "确认删除该条需求？",
      onOk: () => removeRow(r.id)
    }, /*#__PURE__*/React.createElement(Button, {
      size: "mini",
      status: "danger",
      type: "text"
    }, "删除")))
  }];
  return /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Space, {
    style: {
      marginBottom: 12
    }
  }, /*#__PURE__*/React.createElement(Button, {
    onClick: load,
    loading: loading
  }, "刷新")), /*#__PURE__*/React.createElement(Table, {
    columns: columns,
    data: rows,
    rowKey: "id",
    loading: loading,
    pagination: {
      pageSize: 15
    },
    scroll: {
      x: 1200
    },
    noDataElement: /*#__PURE__*/React.createElement(Empty, {
      description: "暂无需求数据"
    })
  }), /*#__PURE__*/React.createElement(Modal, {
    title: "编辑需求 #" + (editing ? editing.id : ""),
    visible: !!editing,
    onCancel: () => setEditing(null),
    onOk: () => editForm.submit(),
    okText: "保存"
  }, /*#__PURE__*/React.createElement(Form, {
    form: editForm,
    layout: "vertical",
    onSubmit: saveEdit
  }, /*#__PURE__*/React.createElement(FormItem, {
    label: "部门",
    field: "department",
    rules: [{
      required: true
    }]
  }, /*#__PURE__*/React.createElement(Select, {
    options: DEPARTMENTS
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "需求人",
    field: "requester",
    rules: [{
      required: true
    }]
  }, /*#__PURE__*/React.createElement(Input, {
    maxLength: 50
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "需求描述",
    field: "description",
    rules: [{
      required: true
    }]
  }, /*#__PURE__*/React.createElement(TextArea, {
    autoSize: {
      minRows: 2
    }
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "应用场景",
    field: "scenario"
  }, /*#__PURE__*/React.createElement(TextArea, {
    autoSize: {
      minRows: 2
    }
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "单据名称",
    field: "bill_name",
    rules: [{
      required: true
    }]
  }, /*#__PURE__*/React.createElement(Input, {
    maxLength: 100
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "需求类型",
    field: "req_type",
    rules: [{
      required: true
    }]
  }, /*#__PURE__*/React.createElement(Select, {
    options: REQ_TYPES
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "紧急程度",
    field: "urgency"
  }, /*#__PURE__*/React.createElement(Select, {
    options: URGENCIES
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "能否实现",
    field: "feasibility"
  }, /*#__PURE__*/React.createElement(Select, {
    options: FEASIBILITIES
  })), /*#__PURE__*/React.createElement(FormItem, {
    label: "备注",
    field: "remark"
  }, /*#__PURE__*/React.createElement(Input, {
    maxLength: 500
  })))));
}

// ---------------- 钉钉通讯录授权（懒加载远程搜索） ----------------
function DingTalkAuth({
  onAuthorized
}) {
  const [options, setOptionsState] = useState([]);
  const [fetching, setFetching] = useState(false);
  const timerRef = useRef(null);
  const optionsRef = useRef([]);
  const setOptions = list => {
    optionsRef.current = list;
    setOptionsState(list);
  };

  // 输入即搜（250ms 防抖），结果进入下拉选项
  const handleSearch = kw => {
    if (timerRef.current) clearTimeout(timerRef.current);
    const kw2 = (kw || "").trim();
    if (!kw2) {
      setOptions([]);
      setFetching(false);
      return;
    }
    setFetching(true);
    timerRef.current = setTimeout(async () => {
      try {
        const data = await window.WBAuth.api("/api/admin/dd_users?name=" + encodeURIComponent(kw2));
        if (data.ok) {
          setOptions((data.users || []).slice(0, 30).map(u => ({
            label: u.name + (u.dept ? "（" + u.dept + "）" : ""),
            value: u.userid,
            user: u
          })));
          if (!(data.users || []).length) Message.info("未找到匹配的员工，请确认姓名");
        } else {
          Message.error(data.error || "搜索失败");
          setOptions([]);
        }
      } catch (e) {
        Message.error("网络异常：" + e.message);
      }
      setFetching(false);
    }, 250);
  };

  // 选中即授权（从 optionsRef 反查用户，避免 Arco 规范化 option 丢自定义字段）
  const handleChange = async value => {
    if (!value) return;
    const u = (optionsRef.current.find(o => o.value === value) || {}).user;
    setOptions([]);
    if (!u) {
      Message.error("未找到该员工信息，请重新搜索");
      return;
    }
    const data = await post("/api/admin/add_admin", {
      userid: u.userid,
      name: u.name
    });
    if (data.ok) {
      Message.success("已授权「" + u.name + "」为管理员");
      if (onAuthorized) onAuthorized();
    } else Message.error(data.error || "授权失败");
  };
  return /*#__PURE__*/React.createElement(Card, {
    title: "从钉钉通讯录授权管理员",
    style: {
      marginBottom: 16
    }
  }, /*#__PURE__*/React.createElement(Select, {
    showSearch: true,
    allowClear: true,
    value: undefined,
    placeholder: "输入员工姓名，边输入边搜索",
    style: {
      width: 320
    },
    loading: fetching,
    filterOption: false,
    onSearch: handleSearch,
    onChange: handleChange,
    options: options,
    notFoundContent: fetching ? "搜索中…" : "输入姓名关键字后自动搜索"
  }), /*#__PURE__*/React.createElement(Typography.Text, {
    type: "secondary",
    style: {
      display: "block",
      marginTop: 8
    }
  }, "懒加载：输入姓名即自动搜索企业内任意员工（不限是否提交过需求），在下拉中点选即完成授权，该员工在钉钉中打开需求列表即自动获得管理模式。"));
}

// ---------------- 管理员设置 ----------------
function AdminSettings() {
  const [admins, setAdmins] = useState([]);
  const [submitters, setSubmitters] = useState([]);
  const [selected, setSelected] = useState(undefined);
  const [pwForm] = Form.useForm();
  const load = async () => {
    const data = await window.WBAuth.api("/api/admin/admins");
    if (data._status === 403) {
      Message.error("权限失效，请重新登录");
      return;
    }
    setAdmins(data.admins || []);
    setSubmitters(data.submitters || []);
  };
  useEffect(() => {
    load();
  }, []);
  const addAdmin = async () => {
    if (!selected) {
      Message.warning("请先选择要指定的人员");
      return;
    }
    const target = submitters.find(s => s.userid === selected);
    const data = await post("/api/admin/add_admin", {
      userid: selected,
      name: target ? target.name : ""
    });
    if (data.ok) {
      Message.success("已设为管理员");
      setSelected(undefined);
      load();
    } else Message.error(data.error || "设置失败");
  };
  const removeAdmin = async id => {
    const data = await post("/api/admin/remove_admin", {
      id
    });
    if (data.ok) {
      Message.success("已移除");
      load();
    } else Message.error(data.error || "移除失败");
  };
  const changePassword = async values => {
    const data = await post("/api/admin/change_password", values);
    if (data.ok) {
      Message.success("密码已修改");
      pwForm.resetFields();
    } else Message.error(data.error || "修改失败");
  };
  const adminColumns = [{
    title: "姓名",
    dataIndex: "name",
    width: 120
  }, {
    title: "钉钉 UserId",
    dataIndex: "userid"
  }, {
    title: "加入时间",
    dataIndex: "created_at",
    width: 160
  }, {
    title: "操作",
    dataIndex: "_op",
    width: 90,
    render: (v, r) => /*#__PURE__*/React.createElement(Popconfirm, {
      title: "确认移除该管理员？",
      onOk: () => removeAdmin(r.id)
    }, /*#__PURE__*/React.createElement(Button, {
      size: "mini",
      status: "danger",
      type: "text"
    }, "移除"))
  }];
  return /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(DingTalkAuth, {
    onAuthorized: load
  }), /*#__PURE__*/React.createElement(Card, {
    title: "当前管理员",
    style: {
      marginBottom: 16
    }
  }, /*#__PURE__*/React.createElement(Table, {
    columns: adminColumns,
    data: admins,
    rowKey: "id",
    pagination: false,
    noDataElement: /*#__PURE__*/React.createElement(Empty, {
      description: "暂无管理员，可从上方钉钉通讯录或下方提交人中指定"
    })
  })), /*#__PURE__*/React.createElement(Card, {
    title: "指定管理员（从已提交过需求的钉钉用户中选择）",
    style: {
      marginBottom: 16
    }
  }, /*#__PURE__*/React.createElement(Space, null, /*#__PURE__*/React.createElement(Select, {
    placeholder: "选择人员",
    style: {
      width: 260
    },
    value: selected,
    onChange: setSelected,
    showSearch: true
  }, submitters.map(s => /*#__PURE__*/React.createElement(Select.Option, {
    key: s.userid,
    value: s.userid
  }, s.name, "（", s.userid, "）"))), /*#__PURE__*/React.createElement(Button, {
    type: "primary",
    onClick: addAdmin
  }, "设为管理员")), /*#__PURE__*/React.createElement(Typography.Text, {
    type: "secondary",
    style: {
      display: "block",
      marginTop: 8
    }
  }, "被指定的人员在钉钉中打开需求列表时，自动获得编辑、删除和评估权限。")), /*#__PURE__*/React.createElement(Card, {
    title: "修改管理密码"
  }, /*#__PURE__*/React.createElement(Form, {
    form: pwForm,
    layout: "vertical",
    onSubmit: changePassword,
    style: {
      maxWidth: 360
    }
  }, /*#__PURE__*/React.createElement(FormItem, {
    label: "原密码",
    field: "old_password",
    rules: [{
      required: true,
      message: "请输入原密码"
    }]
  }, /*#__PURE__*/React.createElement(Input.Password, null)), /*#__PURE__*/React.createElement(FormItem, {
    label: "新密码",
    field: "new_password",
    rules: [{
      required: true,
      message: "请输入新密码"
    }, {
      minLength: 6,
      message: "至少 6 位"
    }]
  }, /*#__PURE__*/React.createElement(Input.Password, null)), /*#__PURE__*/React.createElement(Button, {
    type: "primary",
    htmlType: "submit"
  }, "修改密码"))));
}

// ---------------- 运行日志 ----------------
function LogViewer() {
  const [lines, setLines] = useState([]);
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);
  const load = async () => {
    setLoading(true);
    const data = await window.WBAuth.api("/api/admin/logs");
    if (data.ok) setLines(data.lines || []);
    setLoading(false);
  };
  useEffect(() => {
    load();
  }, []);
  useEffect(() => {
    if (!auto) return;
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, [auto]);
  const clear = async () => {
    const data = await post("/api/admin/logs/clear", {});
    if (data.ok) {
      setLines([]);
      Message.success("日志已清空");
    } else Message.error(data.error || "操作失败");
  };
  return /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Space, {
    style: {
      marginBottom: 12
    }
  }, /*#__PURE__*/React.createElement(Button, {
    onClick: load,
    loading: loading
  }, "刷新"), /*#__PURE__*/React.createElement(Button, {
    onClick: () => setAuto(!auto),
    type: auto ? "primary" : "default"
  }, auto ? "自动刷新：开" : "自动刷新：关"), /*#__PURE__*/React.createElement(Popconfirm, {
    title: "确认清空日志缓冲？",
    onOk: clear
  }, /*#__PURE__*/React.createElement(Button, {
    status: "danger",
    type: "text"
  }, "清空")), /*#__PURE__*/React.createElement(Typography.Text, {
    type: "secondary"
  }, "保留最近 2000 行，每 10 秒自动刷新")), /*#__PURE__*/React.createElement("pre", {
    style: {
      background: "#1d2129",
      color: "#c9d1d9",
      padding: 12,
      borderRadius: 6,
      maxHeight: 520,
      overflow: "auto",
      fontSize: 12,
      lineHeight: "18px",
      whiteSpace: "pre-wrap",
      wordBreak: "break-all"
    }
  }, lines.length ? lines.join("\n") : "暂无日志"));
}

// ---------------- 主入口 ----------------
function App() {
  const [authed, setAuthed] = useState(null); // null=检查中 false=未登录 true=已登录
  const [pwForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [ddState, setDdState] = useState(window.WBAuth.state);
  useEffect(() => {
    return window.WBAuth.subscribe(function (s) {
      setDdState(Object.assign({}, s));
    });
  }, []);
  useEffect(() => {
    (async () => {
      try {
        await window.WBAuth.ensureLogin(); // 钉钉内先免登，管理员身份可直接进入
        const data = await window.WBAuth.api("/api/admin/status");
        setAuthed(!!data.logged);
      } catch (e) {
        setAuthed(false);
      }
    })();
  }, []);
  const login = async values => {
    setLoading(true);
    const data = await post("/api/admin/login", values);
    if (data.ok) {
      if (data.token) window.WBAuth.setToken(data.token);
      setAuthed(true);
      Message.success("登录成功");
    } else Message.error(data.error || "登录失败");
    setLoading(false);
  };
  if (authed === null) return null;
  if (!authed) {
    return /*#__PURE__*/React.createElement("div", {
      className: "login-wrap"
    }, /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement(Typography.Title, {
      heading: 5,
      style: {
        marginBottom: 16
      }
    }, "管理面板登录"), /*#__PURE__*/React.createElement(Form, {
      form: pwForm,
      layout: "vertical",
      onSubmit: login
    }, /*#__PURE__*/React.createElement(FormItem, {
      label: "管理密码",
      field: "password",
      rules: [{
        required: true,
        message: "请输入管理密码"
      }]
    }, /*#__PURE__*/React.createElement(Input.Password, {
      placeholder: "初始密码 admin123，登录后请立即修改"
    })), /*#__PURE__*/React.createElement(Button, {
      type: "primary",
      htmlType: "submit",
      long: true,
      loading: loading
    }, "登 录")), /*#__PURE__*/React.createElement("div", {
      style: {
        marginTop: 12
      }
    }, /*#__PURE__*/React.createElement(Alert, {
      type: "normal",
      content: "如果你已被授权为管理员，在钉钉里打开本页会自动进入，无需输密码。"
    })), ddState.status === "fail" && /*#__PURE__*/React.createElement(Alert, {
      type: "warning",
      style: {
        marginTop: 12
      },
      title: "钉钉免登未成功",
      content: /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("div", null, "原因：", ddState.message || "未知"), ddState.tried && ddState.tried.length > 0 && /*#__PURE__*/React.createElement("div", {
        style: {
          marginTop: 4,
          fontSize: 12,
          color: "#86909c"
        }
      }, "尝试记录：", ddState.tried.join("；")))
    })));
  }
  return /*#__PURE__*/React.createElement("div", {
    className: "wrap"
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      marginBottom: 16
    }
  }, /*#__PURE__*/React.createElement(Typography.Title, {
    heading: 4,
    style: {
      margin: 0
    }
  }, "管理面板"), /*#__PURE__*/React.createElement(Space, null, /*#__PURE__*/React.createElement(Button, {
    href: "/list"
  }, "需求列表"), /*#__PURE__*/React.createElement(Button, {
    href: "/"
  }, "填写页"))), /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement(Tabs, {
    defaultActiveTab: "requirements",
    type: "card"
  }, /*#__PURE__*/React.createElement(TabPane, {
    key: "requirements",
    title: "需求管理"
  }, /*#__PURE__*/React.createElement(RequirementManager, null)), /*#__PURE__*/React.createElement(TabPane, {
    key: "settings",
    title: "管理员设置"
  }, /*#__PURE__*/React.createElement(AdminSettings, null)), /*#__PURE__*/React.createElement(TabPane, {
    key: "logs",
    title: "运行日志"
  }, /*#__PURE__*/React.createElement(LogViewer, null)))));
}
ReactDOM.createRoot(document.getElementById("root")).render(/*#__PURE__*/React.createElement(App, null));

/* ============================================================
 * 需求收集 - 公共鉴权模块
 * 1) 钉钉免登（不依赖 dd.config / dd.ready，自动尝试新旧 API）
 * 2) token 会话（钉钉 webview 内 cookie 不可靠，改存 localStorage）
 * 3) 免登失败时暴露可读的错误原因，供页面展示
 * ============================================================ */
(function (global) {
  "use strict";

  var TOKEN_KEY = "dingtalk_req_token";

  function lsGet(k) { try { return global.localStorage.getItem(k); } catch (e) { return null; } }
  function lsSet(k, v) { try { global.localStorage.setItem(k, v); } catch (e) {} }
  function lsDel(k) { try { global.localStorage.removeItem(k); } catch (e) {} }

  function errText(e) {
    if (!e) return "未知错误";
    if (typeof e === "string") return e;
    return e.errorMessage || e.errMsg || e.message || String(e).slice(0, 150);
  }

  /* ---------- token ---------- */
  function getToken() { return lsGet(TOKEN_KEY) || ""; }
  function setToken(t) { if (t) lsSet(TOKEN_KEY, t); }
  function clearToken() { lsDel(TOKEN_KEY); }

  /* ---------- 统一请求封装（自动带 token） ---------- */
  function api(url, options) {
    options = options || {};
    var headers = {};
    var k;
    if (options.headers) { for (k in options.headers) { headers[k] = options.headers[k]; } }
    var t = getToken();
    if (t) headers["X-Auth-Token"] = t;
    if (options.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
    return fetch(url, {
      method: options.method || "GET",
      headers: headers,
      body: options.body,
      credentials: "same-origin"
    }).then(function (r) {
      return r.json().catch(function () { return {}; }).then(function (d) {
        d = d || {};
        d._status = r.status;
        return d;
      });
    });
  }

  /* ---------- 钉钉环境 ---------- */
  function ddSdk() { return global.dd || null; }

  function inDingTalk() {
    var d = ddSdk();
    if (!d) return false;
    try {
      if (d.env && d.env.platform) return d.env.platform !== "notInDingTalk";
    } catch (e) {}
    return /DingTalk/i.test(global.navigator.userAgent || "");
  }

  function envInfo() {
    var d = ddSdk(), out = [];
    if (!d) return "未加载钉钉 JSAPI";
    try { out.push("platform=" + (d.env && d.env.platform)); } catch (e) {}
    try { out.push("version=" + ((d.env && d.env.version) || d.version || "?")); } catch (e) {}
    out.push("getAuthCode=" + (typeof d.getAuthCode));
    out.push("requestAuthCode=" + (typeof d.requestAuthCode));
    out.push("legacy=" + !!(d.runtime && d.runtime.permission && d.runtime.permission.requestAuthCode));
    return out.join(" · ");
  }

  /* 兼容回调式 / Promise 式调用，带超时保护 */
  function callDd(invoke) {
    return new Promise(function (resolve, reject) {
      var settled = false;
      function ok(v) { if (!settled) { settled = true; resolve(v); } }
      function bad(e) { if (!settled) { settled = true; reject(e); } }
      try {
        var ret = invoke({ onSuccess: ok, onFail: bad });
        if (ret && typeof ret.then === "function") { ret.then(ok, bad); }
      } catch (e) { bad(e); }
      global.setTimeout(function () { bad(new Error("JSAPI 无响应（超时 10s）")); }, 10000);
    });
  }

  /* 依次尝试 3 套免登 API，返回 {code, via, tried} */
  function requestAuthCode(cfg) {
    var d = ddSdk();
    var tried = [];
    var order = [];

    if (d && typeof d.getAuthCode === "function") {
      order.push(["dd.getAuthCode", function () {
        return callDd(function (cb) {
          return d.getAuthCode({ corpId: cfg.corpId, clientId: cfg.clientId, onSuccess: cb.onSuccess, onFail: cb.onFail });
        });
      }]);
    }
    if (d && typeof d.requestAuthCode === "function") {
      order.push(["dd.requestAuthCode", function () {
        return callDd(function (cb) {
          return d.requestAuthCode({ corpId: cfg.corpId, clientId: cfg.clientId, onSuccess: cb.onSuccess, onFail: cb.onFail });
        });
      }]);
    }
    if (d && d.runtime && d.runtime.permission && typeof d.runtime.permission.requestAuthCode === "function") {
      order.push(["dd.runtime.permission.requestAuthCode", function () {
        var legacy = d.runtime.permission.requestAuthCode;
        return callDd(function (cb) {
          return legacy.call(d.runtime.permission, { corpId: cfg.corpId, onSuccess: cb.onSuccess, onFail: cb.onFail });
        });
      }]);
    }

    if (!order.length) {
      return Promise.resolve({ code: "", via: "", tried: ["钉钉 JSAPI 未提供任何免登 API"] });
    }

    var i = 0;
    function next() {
      if (i >= order.length) { return Promise.resolve({ code: "", via: "", tried: tried }); }
      var item = order[i++];
      return item[1]().then(function (r) {
        var code = (r && (r.authCode || r.code)) || "";
        if (code) { return { code: code, via: item[0], tried: tried }; }
        tried.push(item[0] + " → 返回空授权码");
        return next();
      })["catch"](function (e) {
        tried.push(item[0] + " → " + errText(e));
        return next();
      });
    }
    return next();
  }

  function waitDdReady() {
    var d = ddSdk();
    if (!d || typeof d.ready !== "function") { return Promise.resolve(); }
    return new Promise(function (resolve) {
      var done = false;
      function fin() { if (!done) { done = true; resolve(); } }
      try { d.ready(fin); } catch (e) {}
      global.setTimeout(fin, 2500);
    });
  }

  /* ---------- 登录态 ---------- */
  var state = {
    status: "unknown",   // unknown | ok | logging | fail | outside
    message: "",
    via: "",
    tried: [],
    env: ""
  };
  var subs = [];

  function notify() { subs.forEach(function (fn) { try { fn(state); } catch (e) {} }); }
  function setState(patch) {
    var k; for (k in patch) { state[k] = patch[k]; }
    notify();
  }
  function subscribe(fn) { subs.push(fn); return function () { subs = subs.filter(function (f) { return f !== fn; }); }; }

  function ensureLogin(force) {
    setState({ env: envInfo() });
    return api("/api/me").then(function (me) {
      if (me.logged && !force) {
        var m = { logged: true, name: me.name, department: me.department, userid: me.userid, is_admin: me.is_admin, panel_admin: me.panel_admin };
        setState({ status: "ok", message: "" });
        return m;
      }
      if (!inDingTalk()) {
        setState({ status: "outside", message: "当前不在钉钉客户端内，可手动填写姓名" });
        return me;
      }
      setState({ status: "logging", message: "正在钉钉免登…", tried: [] });
      return fetch("/api/dingtalk/config").then(function (r) { return r.json(); }).then(function (cfg) {
        if (!cfg.enabled) {
          setState({ status: "fail", message: "服务端未配置企业 CorpId" });
          return me;
        }
        return waitDdReady().then(function () { return requestAuthCode(cfg); }).then(function (res) {
          if (!res.code) {
            setState({ status: "fail", message: "获取免登授权码失败", via: res.via, tried: res.tried });
            return me;
          }
          setState({ status: "logging", message: "正在验证身份…", via: res.via, tried: res.tried });
          return api("/api/dingtalk/login", {
            method: "POST",
            body: JSON.stringify({ code: res.code })
          }).then(function (d) {
            if (!d.ok) {
              setState({ status: "fail", message: d.error || "服务端免登失败", via: res.via, tried: res.tried });
              return me;
            }
            setToken(d.token);
            setState({ status: "ok", message: "", via: res.via, tried: res.tried });
            return { logged: true, name: d.name, department: d.department, userid: d.userid, is_admin: d.is_admin };
          });
        });
      });
    })["catch"](function (e) {
      setState({ status: "fail", message: "请求异常：" + errText(e) });
      return { logged: false };
    });
  }

  function logout() {
    clearToken();
    return api("/api/logout", { method: "POST" }).then(function () {
      setState({ status: "outside", message: "" });
    });
  }

  /* ---------- 附件辅助 ---------- */
  function formatSize(n) {
    if (n == null) return "";
    if (n < 1024) return n + "B";
    if (n < 1048576) return (n / 1024).toFixed(1) + "KB";
    return (n / 1048576).toFixed(2) + "MB";
  }
  function downloadURL(id) {
    return "/api/attachment/download?id=" + id + "&token=" + encodeURIComponent(getToken());
  }
  /* 上传附件（multipart，专用封装：不能手动设置 Content-Type）
     带超时保护，避免钉钉 webview 里请求悬停导致页面一直转圈 */
  function uploadFile(reqId, file, timeoutMs) {
    var fd = new FormData();
    fd.append("req_id", reqId);
    fd.append("file", file);
    var ctl = typeof AbortController === "function" ? new AbortController() : null;
    var timer = null;
    var req = fetch("/api/attachment/upload", {
      method: "POST",
      headers: { "X-Auth-Token": getToken() },
      body: fd,
      credentials: "same-origin",
      signal: ctl ? ctl.signal : undefined
    }).then(function (r) {
      if (timer) clearTimeout(timer);
      return r.json().catch(function () { return {}; });
    });
    if (ctl) {
      timer = setTimeout(function () { ctl.abort(); }, timeoutMs || 60000);
    }
    return req.catch(function (e) {
      if (timer) clearTimeout(timer);
      return { ok: false, error: "上传超时或网络中断（文件可能已上传，请到管理面板确认）" };
    });
  }

  global.WBAuth = {
    api: api,
    ensureLogin: ensureLogin,
    logout: logout,
    inDingTalk: inDingTalk,
    envInfo: envInfo,
    getToken: getToken,
    setToken: setToken,
    clearToken: clearToken,
    subscribe: subscribe,
    state: state,
    setState: setState,
    errText: errText,
    formatSize: formatSize,
    downloadURL: downloadURL,
    uploadFile: uploadFile
  };
})(window);

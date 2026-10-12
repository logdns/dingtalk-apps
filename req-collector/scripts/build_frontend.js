#!/usr/bin/env node
/**
 * 前端构建脚本：src/*.jsx  ->  static/*.js
 *
 * 说明：
 *  - 浏览器端运行时没有打包器，页面直接 <script src="/static/xxx.js">，
 *    所以这里用 Babel（static/babel.min.js，UMD，可在 Node 中直接 require）
 *    把 JSX 预编译为 React.createElement 形式的普通 JS。
 *  - 编译选项固定为 classic runtime（依赖全局 React），与页面加载方式一致。
 *  - src/auth.js 是纯 JS（不含 JSX），直接复制到 static/auth.js。
 *
 * 用法：node scripts/build_frontend.js
 */
const fs = require("fs");
const path = require("path");

const ROOT = path.resolve(__dirname, "..");
const SRC_DIR = path.join(ROOT, "src");
const OUT_DIR = path.join(ROOT, "static");
const BABEL_FILE = path.join(OUT_DIR, "babel.min.js");

const Babel = require(BABEL_FILE);

const BANNER = (file) =>
  "/*!\n" +
  " * 本文件由 src/" + file + " 自动编译生成（node scripts/build_frontend.js）。\n" +
  " * 请勿直接修改本文件，改源码后重新编译，并更新 handlers.go 中的 staticVer 以刷新钉钉缓存。\n" +
  " */\n";

let count = 0;

// 1) JSX -> JS
for (const file of fs.readdirSync(SRC_DIR).filter((f) => f.endsWith(".jsx"))) {
  const srcPath = path.join(SRC_DIR, file);
  const outName = file.replace(/\.jsx$/, ".js");
  const outPath = path.join(OUT_DIR, outName);

  const code = fs.readFileSync(srcPath, "utf8");
  const out = Babel.transform(code, {
    presets: [["react", { runtime: "classic" }]],
    filename: file,
    compact: false,
    comments: true,
  });

  fs.writeFileSync(outPath, BANNER(file) + out.code + "\n", "utf8");
  console.log("build  src/" + file + "  ->  static/" + outName);
  count++;
}

// 2) 纯 JS 直接同步
for (const file of fs.readdirSync(SRC_DIR).filter((f) => f.endsWith(".js"))) {
  const srcPath = path.join(SRC_DIR, file);
  const outPath = path.join(OUT_DIR, file);
  fs.copyFileSync(srcPath, outPath);
  console.log("copy   src/" + file + "  ->  static/" + file);
  count++;
}

console.log("done: " + count + " file(s) -> static/");

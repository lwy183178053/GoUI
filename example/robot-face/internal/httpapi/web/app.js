import {requestJSON} from "/api.js";

// 名称列表只创建一次；状态轮询修改已有控件，预览独立更新。
const list = document.querySelector("#expressions");
const loop = document.querySelector("#loop");
const interrupt = document.querySelector("#interrupt");
const message = document.querySelector("#message");
const connection = document.querySelector("#connection");
const catalog = new Map();
let selectedName = "normal";
let initialized = false;
let sending = false;
let revision = 0;

function updateExample() {
  document.querySelector("#api-example").textContent = JSON.stringify({
    name: selectedName, loop: loop.checked, interrupt: interrupt.checked,
  });
}

function buildControls(expressions) {
  for (const expression of expressions) {
    catalog.set(expression.name, expression);
    const button = document.createElement("button");
    button.type = "button";
    button.className = "expression";
    button.dataset.name = expression.name;
    button.textContent = expression.label;
    button.title = expression.name;
    button.addEventListener("click", () => sendExpression(expression.name));
    list.append(button);
  }
  initialized = true;
}

function render(data) {
  const {expression, pending} = data.current;
  document.querySelector("#expression-name").textContent = expression.label;
  document.querySelector("#expression-code").textContent = expression.name;
  const nextLabel = pending ? catalog.get(pending.name)?.label || pending.name : "";
  document.querySelector("#playback-state").textContent = pending
    ? `本轮结束后 → ${nextLabel}`
    : expression.name === "normal" ? "正常 · 每隔 2～5 秒随机眨眼"
    : data.current.loop ? "持续循环" : "播放一次 · 结束后自动回正常";
  for (const button of list.querySelectorAll("button")) {
    button.setAttribute("aria-pressed", String(button.dataset.name === expression.name));
    button.classList.toggle("is-pending", button.dataset.name === pending?.name);
  }
  message.textContent = pending ? `正在播放${expression.label}，等待切换到${nextLabel}。` : `当前：${expression.label}。`;
  message.dataset.error = "false";
  connection.textContent = "设备已连接";
  connection.dataset.status = "connected";
}

function showError(error) {
  connection.textContent = "设备未连接";
  connection.dataset.status = "disconnected";
  message.textContent = error.message;
  message.dataset.error = "true";
}

async function sendExpression(name) {
  if (sending) return;
  selectedName = name;
  updateExample();
  sending = true;
  ++revision;
  try { render(await requestJSON("/api/emotion", {name, loop: loop.checked, interrupt: interrupt.checked})); }
  catch (error) { showError(error); }
  finally { sending = false; }
}

loop.addEventListener("change", updateExample);
interrupt.addEventListener("change", updateExample);

async function poll() {
  if (!sending) {
    const started = revision;
    try {
      if (!initialized) buildControls((await requestJSON("/api/expressions")).expressions);
      const data = await requestJSON("/api/emotion");
      if (started === revision) render(data);
    } catch (error) { if (started === revision) showError(error); }
  }
  setTimeout(poll, 500);
}
updateExample();
poll();

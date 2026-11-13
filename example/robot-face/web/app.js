"use strict";

const matrix = document.querySelector("#matrix");
const catalog = document.querySelector("#catalog");
const form = document.querySelector("#vector-form");
const valence = document.querySelector("#valence");
const arousal = document.querySelector("#arousal");
const message = document.querySelector("#message");
const connection = document.querySelector("#connection");
const preview = document.querySelector("#preview");
let initialized = false;
let sending = false;
let requestID = 0;

function setConnection(connected) {
  connection.textContent = connected ? "设备已连接" : "设备未连接";
  connection.dataset.status = connected ? "connected" : "disconnected";
}

function setMessage(text, error = false) {
  message.textContent = text;
  message.dataset.error = String(error);
}

function updateInputLabel() {
  document.querySelector("#vector-value").textContent = `[${valence.value}, ${arousal.value}]`;
}

function buildControls(expressions) {
  const anchors = new Map(expressions.map(item => [`${item.valence},${item.arousal}`, item]));
  for (let y = 10; y >= 0; y--) {
    const tick = document.createElement("span");
    tick.className = "tick";
    tick.textContent = y;
    matrix.append(tick);
    for (let x = 0; x <= 10; x++) {
      const item = anchors.get(`${x},${y}`);
      const button = document.createElement("button");
      button.type = "button";
      button.className = "cell";
      button.dataset.valence = x;
      button.dataset.arousal = y;
      button.setAttribute("aria-pressed", "false");
      button.setAttribute("aria-label", `情绪值 ${x}，唤醒度 ${y}${item ? `，${item.label}` : ""}`);
      if (item) {
        button.textContent = item.label;
        button.dataset.expression = item.name;
      }
      button.addEventListener("click", () => sendVector(x, y));
      matrix.append(button);
    }
  }
  matrix.append(document.createElement("span"));
  for (let x = 0; x <= 10; x++) {
    const tick = document.createElement("span");
    tick.className = "tick";
    tick.textContent = x;
    matrix.append(tick);
  }
  for (const item of expressions) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "preset";
    button.dataset.expression = item.name;
    button.setAttribute("aria-pressed", "false");
    button.append(document.createTextNode(item.label));
    const coordinate = document.createElement("span");
    coordinate.textContent = `${item.valence}, ${item.arousal}`;
    button.append(coordinate);
    button.addEventListener("click", () => sendVector(item.valence, item.arousal));
    catalog.append(button);
  }
  document.querySelector("#expression-count").textContent = `${expressions.length} 张 GIF · 点击直达`;
  initialized = true;
}

function render(data) {
  if (!initialized) buildControls(data.expressions);
  const {vector, expression} = data.current;
  document.querySelector("#expression-name").textContent = expression.label;
  document.querySelector("#expression-code").textContent = expression.name;
  // 轮询时保留正在编辑的输入，避免用户输入到一半被设备状态覆盖。
  if (document.activeElement !== valence && document.activeElement !== arousal) {
    valence.value = vector.valence;
    arousal.value = vector.arousal;
    updateInputLabel();
  }
  if (preview.dataset.expression !== expression.name) {
    preview.src = `/gif/${encodeURIComponent(expression.file)}`;
    preview.dataset.expression = expression.name;
    preview.hidden = false;
  }
  for (const button of matrix.querySelectorAll("button")) {
    const isInput = Number(button.dataset.valence) === vector.valence && Number(button.dataset.arousal) === vector.arousal;
    button.setAttribute("aria-pressed", String(isInput));
    button.classList.toggle("is-match", button.dataset.expression === expression.name);
  }
  for (const button of catalog.querySelectorAll("button")) {
    button.setAttribute("aria-pressed", String(button.dataset.expression === expression.name));
  }
  document.querySelector("#api-example").textContent = [
    `curl -X POST '${location.origin}/api/emotion' \\`,
    "  -H 'Content-Type: application/json' \\",
    `  -d '{"valence":${vector.valence},"arousal":${vector.arousal}}'`,
  ].join("\n");
  setMessage(`输入 [${vector.valence}, ${vector.arousal}] → ${expression.label} · 素材坐标 [${expression.valence}, ${expression.arousal}]`);
  setConnection(true);
}

async function readResponse(response) {
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || `请求失败：${response.status}`);
  return data;
}

async function refresh() {
  if (sending) return;
  const id = ++requestID;
  try {
    const response = await fetch("/api/emotion", {cache: "no-store", signal: AbortSignal.timeout(5000)});
    const data = await readResponse(response);
    // 较早发出的轮询结果不能覆盖刚刚提交的新状态。
    if (id === requestID) render(data);
  } catch (error) {
    if (id === requestID) {
      setConnection(false);
      setMessage(`连接失败：${error.message}。将自动重试。`, true);
    }
  }
}

async function sendVector(x, y) {
  if (sending) return;
  if (!Number.isInteger(x) || !Number.isInteger(y) || x < 0 || x > 10 || y < 0 || y > 10) {
    setMessage("请输入 0～10 的整数。", true);
    return;
  }
  sending = true;
  ++requestID;
  valence.value = x;
  arousal.value = y;
  updateInputLabel();
  const buttons = document.querySelectorAll(".cell, .preset, #apply");
  buttons.forEach(button => { button.disabled = true; });
  try {
    const response = await fetch("/api/emotion", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({valence: x, arousal: y}),
      signal: AbortSignal.timeout(5000),
    });
    render(await readResponse(response));
  } catch (error) {
    setConnection(false);
    setMessage(`应用失败：${error.message}`, true);
  } finally {
    sending = false;
    buttons.forEach(button => { button.disabled = false; });
  }
}

form.addEventListener("submit", event => {
  event.preventDefault();
  sendVector(Number(valence.value), Number(arousal.value));
});
valence.addEventListener("input", updateInputLabel);
arousal.addEventListener("input", updateInputLabel);

// 只同步状态，不重复提交向量；大模型从其它客户端修改时面板也会更新。
async function poll() {
  await refresh();
  setTimeout(poll, 1000);
}
poll();

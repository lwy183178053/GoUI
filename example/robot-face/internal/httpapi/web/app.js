"use strict";

// 状态轮询只修改已有控件，预览由 preview.js 独立更新。
(() => {
  const matrix = document.querySelector("#matrix");
  const form = document.querySelector("#vector-form");
  const valence = document.querySelector("#valence");
  const arousal = document.querySelector("#arousal");
  const message = document.querySelector("#message");
  const connection = document.querySelector("#connection");
  let gridSize = 5;
  let initialized = false;
  let sending = false;
  let revision = 0;

  function inputLabel() {
    document.querySelector("#vector-value").textContent = `[${valence.value}, ${arousal.value}]`;
  }

  function buildControls(expressions) {
    const anchors = new Map(expressions.map(item => [`${item.valence},${item.arousal}`, item]));
    for (let y = gridSize - 1; y >= 0; y--) {
      const tick = document.createElement("span");
      tick.className = "tick";
      tick.textContent = y;
      matrix.append(tick);
      for (let x = 0; x < gridSize; x++) {
        const item = anchors.get(`${x},${y}`);
        const button = document.createElement("button");
        button.type = "button";
        button.className = "cell";
        button.dataset.valence = x;
        button.dataset.arousal = y;
        button.dataset.expression = item?.name || "";
        button.textContent = item?.label || "就近";
        button.setAttribute("aria-label", `情绪值 ${x}，唤醒度 ${y}，${button.textContent}`);
        button.addEventListener("click", () => sendVector(x, y));
        matrix.append(button);
      }
    }
    matrix.append(document.createElement("span"));
    for (let x = 0; x < gridSize; x++) {
      const tick = document.createElement("span");
      tick.className = "tick";
      tick.textContent = x;
      matrix.append(tick);
    }
    initialized = true;
  }

  function render(data) {
    gridSize = data.grid_size;
    if (!initialized) buildControls(data.expressions);
    const {vector, expression, target, transitioning} = data.current;
    document.querySelector("#expression-name").textContent = expression.label;
    document.querySelector("#expression-code").textContent = expression.name;
    document.querySelector("#playback-state").textContent = transitioning
      ? `正在回正 → ${target.label}` : "保持或循环当前表情";
    if (document.activeElement !== valence && document.activeElement !== arousal) {
      valence.value = vector.valence;
      arousal.value = vector.arousal;
      inputLabel();
    }
    for (const button of matrix.querySelectorAll("button")) {
      const selected = Number(button.dataset.valence) === vector.valence && Number(button.dataset.arousal) === vector.arousal;
      button.setAttribute("aria-pressed", String(selected));
      button.classList.toggle("is-match", button.dataset.expression === target.name);
    }
    message.textContent = `[${vector.valence}, ${vector.arousal}] → ${target.label}${transitioning ? " · 等待回正" : ""}`;
    message.dataset.error = "false";
    document.querySelector("#api-example").textContent = JSON.stringify(vector);
    connection.textContent = "设备已连接";
    connection.dataset.status = "connected";
  }

  async function request(update) {
    const response = await fetch("/api/emotion", {
      method: update === undefined ? "GET" : "POST",
      headers: {"Content-Type": "application/json"},
      body: update === undefined ? undefined : JSON.stringify(update),
      cache: "no-store", signal: AbortSignal.timeout(5000),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || `请求失败：${response.status}`);
    return data;
  }

  function showError(error) {
    connection.textContent = "设备未连接";
    connection.dataset.status = "disconnected";
    message.textContent = error.message;
    message.dataset.error = "true";
  }

  async function sendVector(x, y) {
    if (sending) return;
    if (!Number.isInteger(x) || !Number.isInteger(y) || x < 0 || x >= gridSize || y < 0 || y >= gridSize) {
      message.textContent = "请输入 0～4 的整数。";
      message.dataset.error = "true";
      return;
    }
    sending = true;
    ++revision;
    try { render(await request({valence: x, arousal: y})); }
    catch (error) { showError(error); }
    finally { sending = false; }
  }

  form.addEventListener("submit", event => {
    event.preventDefault();
    sendVector(Number(valence.value), Number(arousal.value));
  });
  valence.addEventListener("input", inputLabel);
  arousal.addEventListener("input", inputLabel);

  async function poll() {
    if (!sending) {
      const started = revision;
      try {
        const data = await request();
        if (started === revision) render(data);
      } catch (error) { if (started === revision) showError(error); }
    }
    setTimeout(poll, 500);
  }
  poll();
})();

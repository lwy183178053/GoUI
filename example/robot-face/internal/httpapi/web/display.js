import {requestJSON} from "/api.js";

// 一次提交完整文字，设备负责逐字显示；轮询只读取进度并保留本地草稿。
const form = document.querySelector("#display-form");
const text = document.querySelector("#display-text");
const message = document.querySelector("#display-message");
let busy = false;
let dirty = false;
let revision = 0;

function render(data) {
  const state = data.current;
  // 编辑框始终显示完整句子，设备上的半截文字不会覆盖草稿。
  if (!dirty && document.activeElement !== text) text.value = state.text;
  message.textContent = state.typing ? "正在逐字显示…" : state.text ? "文字已显示完成。" : "文字已清空。";
  message.dataset.error = "false";
}

async function send(value) {
  if (busy) return;
  busy = true;
  ++revision;
  const controls = form.querySelectorAll("button, textarea");
  controls.forEach(control => { control.disabled = true; });
  try {
    const data = await requestJSON("/api/display", {text: value});
    dirty = false;
    text.value = data.current.text;
    render(data);
  } catch (error) {
    dirty = true; // 失败时保留原文，方便直接重试。
    message.textContent = `发送失败：${error.message}`;
    message.dataset.error = "true";
  } finally {
    busy = false;
    controls.forEach(control => { control.disabled = false; });
  }
}

text.addEventListener("input", () => { dirty = true; });
form.addEventListener("submit", event => {
  event.preventDefault();
  send(text.value);
});
document.querySelector("#display-clear").addEventListener("click", () => send(""));

async function poll() {
  if (!busy) {
    const request = revision;
    try {
      const data = await requestJSON("/api/display");
      if (request === revision) render(data);
    } catch (error) {
      if (request === revision) {
        message.textContent = `状态读取失败：${error.message}`;
        message.dataset.error = "true";
      }
    }
  }
  setTimeout(poll, 500);
}
poll();

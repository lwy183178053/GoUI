"use strict";

// 所有文字共用一个区域，文字请求与表情请求分别维护等待状态。
(() => {
  const form = document.querySelector("#display-form");
  const text = document.querySelector("#display-text");
  const delta = document.querySelector("#display-delta");
  const message = document.querySelector("#display-message");
  let busy = false;
  let dirty = false;
  let revision = 0;

  function render(data) {
    // 没有未提交的编辑时同步设备值；保留正在输入的草稿。
    if (!dirty && document.activeElement !== text) text.value = data.current.text;
  }

  // 提交全文、清空或流式发送成功后，才用设备结果替换编辑内容。
  function syncEditor(data) {
    dirty = false;
    text.value = data.current.text;
  }

  async function call(update) {
    const response = await fetch("/api/display", {
      method: update === undefined ? "GET" : "POST",
      headers: {"Content-Type": "application/json"},
      body: update === undefined ? undefined : JSON.stringify(update),
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || `请求失败：${response.status}`);
    return data;
  }

  async function perform(action) {
    if (busy) return;
    busy = true;
    ++revision;
    form.querySelectorAll("button, input, textarea").forEach(control => { control.disabled = true; });
    try {
      await action();
      message.textContent = "已更新设备底部文字。";
      message.dataset.error = "false";
    } catch (error) {
      message.textContent = `更新失败：${error.message}`;
      message.dataset.error = "true";
    } finally {
      busy = false;
      form.querySelectorAll("button, input, textarea").forEach(control => { control.disabled = false; });
    }
  }

  text.addEventListener("input", () => { dirty = true; });

  form.addEventListener("submit", event => {
    event.preventDefault();
    perform(async () => syncEditor(await call({text: text.value})));
  });
  document.querySelector("#display-append").addEventListener("click", () => {
    perform(async () => {
      const data = await call({append: delta.value});
      render(data);
      delta.value = "";
    });
  });
  document.querySelector("#display-clear").addEventListener("click", () => {
    perform(async () => syncEditor(await call({text: ""})));
  });
  document.querySelector("#display-stream").addEventListener("click", () => {
    const characters = Array.from(text.value);
    perform(async () => {
      // 发送失败时仍保留完整原文，轮询不能用半截结果覆盖草稿。
      dirty = true;
      let data = await call({text: ""});
      // 与模型客户端相同：按生成顺序逐段发送，等待上一段成功后再提交下一段。
      for (let i = 0; i < characters.length; i += 8) {
        data = await call({append: characters.slice(i, i + 8).join("")});
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      syncEditor(data);
    });
  });

  async function poll() {
    if (!busy) {
      const request = revision;
      try {
        const data = await call();
        if (request === revision) render(data);
      } catch (error) {
        if (request === revision) {
          message.textContent = `状态读取失败：${error.message}`;
          message.dataset.error = "true";
        }
      }
    }
    setTimeout(poll, 1000);
  }
  poll();
})();

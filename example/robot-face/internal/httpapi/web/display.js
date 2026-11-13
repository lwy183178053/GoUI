"use strict";

// 回答控件独立维护，避免与表情请求的等待状态相互干扰。
(() => {
  const form = document.querySelector("#display-form");
  const text = document.querySelector("#display-text");
  const status = document.querySelector("#display-status");
  const delta = document.querySelector("#display-delta");
  const message = document.querySelector("#display-message");
  const labels = {idle: "待命", listening: "正在倾听", thinking: "正在思考", speaking: "正在回答"};
  let busy = false;
  let initialized = false;
  let revision = 0;

  function render(data) {
    document.querySelector("#device-text").textContent = data.current.text || "—";
    document.querySelector("#device-status").textContent = labels[data.current.status];
    if (!initialized) {
      if (document.activeElement !== text) text.value = data.current.text;
      if (document.activeElement !== status) status.value = data.current.status;
      initialized = true;
    }
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
    form.querySelectorAll("button, input, select, textarea").forEach(control => { control.disabled = true; });
    try {
      await action();
      message.textContent = "已更新设备底部文字。";
      message.dataset.error = "false";
    } catch (error) {
      message.textContent = `更新失败：${error.message}`;
      message.dataset.error = "true";
    } finally {
      busy = false;
      form.querySelectorAll("button, input, select, textarea").forEach(control => { control.disabled = false; });
    }
  }

  form.addEventListener("submit", event => {
    event.preventDefault();
    perform(async () => render(await call({text: text.value, status: status.value})));
  });
  document.querySelector("#display-append").addEventListener("click", () => {
    perform(async () => {
      const data = await call({append: delta.value});
      render(data);
      text.value = data.current.text;
      delta.value = "";
    });
  });
  document.querySelector("#display-clear").addEventListener("click", () => {
    perform(async () => {
      render(await call({text: "", status: "idle"}));
      text.value = "";
      status.value = "idle";
    });
  });
  document.querySelector("#display-stream").addEventListener("click", () => {
    const characters = Array.from(text.value);
    perform(async () => {
      render(await call({text: "", status: "speaking"}));
      // 与模型客户端相同：按生成顺序逐段发送，等待上一段成功后再提交下一段。
      for (let i = 0; i < characters.length; i += 8) {
        render(await call({append: characters.slice(i, i + 8).join("")}));
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      render(await call({status: "idle"}));
      status.value = "idle";
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

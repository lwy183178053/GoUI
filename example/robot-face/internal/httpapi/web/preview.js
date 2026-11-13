"use strict";

// 保留同一个 Canvas，完整解码新帧后再覆盖，切换表情不会清空或重建预览。
// 画面来自 UI 最近绘制的帧，动作、随机眨眼、文字由同一个播放器决定。
(() => {
  const canvas = document.querySelector("#preview");
  const context = canvas.getContext("2d");
  const state = document.querySelector("#preview-state");

  async function refresh() {
    const started = performance.now();
    let interval = 50;
    if (!document.hidden) {
      try {
        const response = await fetch("/api/preview", {cache: "no-store", signal: AbortSignal.timeout(2000)});
        if (!response.ok) throw new Error("画面暂不可用");
        const bitmap = await createImageBitmap(await response.blob());
        if (canvas.width !== bitmap.width || canvas.height !== bitmap.height) {
          canvas.width = bitmap.width;
          canvas.height = bitmap.height;
        }
        context.drawImage(bitmap, 0, 0);
        bitmap.close();
        state.textContent = "实时画面";
      } catch {
        state.textContent = "预览重连中…";
        interval = 1000;
      }
    } else {
      interval = 500;
    }
    // 串行请求，不积压旧帧；隐藏页面时暂停取帧，失败保留最后画面。
    setTimeout(refresh, Math.max(0, interval - (performance.now() - started)));
  }
  refresh();
})();

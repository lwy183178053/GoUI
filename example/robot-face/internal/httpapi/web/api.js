// 文字与表情共用 JSON 请求规则；画面预览仍按二进制图片单独读取。
export async function requestJSON(path, update) {
  const response = await fetch(path, {
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

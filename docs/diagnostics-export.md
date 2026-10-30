# 诊断包导出（Diagnostic Export）

日志页「诊断包」按钮一键导出用于 bug 报告的 zip。导出前会弹出预览，列出包含的文件与已脱敏的字段；导出只读取运行状态与配置文件，失败不影响正在运行的代理。

## 包格式

zip 写入日志目录（与应用日志同级），文件名 `snishaper-diagnostics-<时间戳>.zip`，包含：

| 文件 | 内容 |
|---|---|
| `diagnostics.json` | 导出时间、版本、平台、服务/代理/系统代理/TUN 状态、自检结果、文件清单 |
| `settings.json` | `config/settings.json` 的**脱敏副本** |
| `rules.json` | `rules/config.json` 的**脱敏副本** |
| `logs.txt` | 最近 500 行应用内存日志（脱敏后） |

`diagnostics.json` 顶层结构：

```json
{
  "generated_at": "RFC3339 时间戳",
  "version": "应用版本",
  "diagnostics": { "version": "...", "platform": "...", "system_proxy": {...}, "tun": {...} },
  "self_check": { "ok": true, "checks": [ { "name": "service", "ok": true } ] },
  "files": [ { "name": "settings.json", "size": 123, "description": "..." } ]
}
```

## 脱敏规则

导出前对配置副本与日志文本执行以下脱敏（源文件永不被修改）：

- **敏感键**：键名匹配 `api_key` / `apikey` / `token` / `secret` / `password` / `authorization`（大小写不敏感，子串匹配）的字符串值替换为 `[REDACTED]`。
- **URL 内嵌凭据**：`scheme://user:pass@host` 中的 userinfo 替换为 `[REDACTED]`。
- **查询参数令牌**：`?api_key=…&token=…&secret=…&password=…` 的值替换为 `[REDACTED]`。
- **Bearer 令牌**：`Bearer <token>` 的令牌部分替换为 `[REDACTED]`。
- 无法解析的配置文件按纯文本执行相同文本脱敏后原样包含（标记问题而不是静默丢弃）。

## 已知限制

- 脱敏基于字段名与常见凭据模式；**导出前请自行确认** zip 内不含其他敏感信息再分享。
- 域名、规则、IP 列表属于诊断必需信息，不做脱敏。

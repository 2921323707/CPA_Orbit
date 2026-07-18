# ADR-0001: 以订阅归档作为多 Provider 运行池唯一事实源

## Status

Accepted

## Context

此前 `k12/` 订阅归档与 `cpa/auths/` 运行目录是两套独立数据源。迁移旧 CPA 时，认证文件直接进入运行目录，但没有生成订阅记录，导致列表与活动池数量不一致，也无法可靠处理删除、重建和其他 Provider（Claude、Kimi、DeepSeek 等）。

## Decision

- 订阅归档是唯一事实源；每个归档 JSON 保留 `provider`/`type` 等 Provider 身份。
- `cpa/auths/` 是可重建的运行副本，不再作为独立账号库。
- Monitor 启动时从订阅归档重建运行副本，并清理无法匹配到订阅归档的运行文件。
- 导入和手动同步按 Provider 保留身份；Codex 继续支持额度查询，其他 Provider 只读取 CPA 管理状态，不调用 Codex 专用额度接口。
- 删除订阅时联动删除匹配的运行副本；存在歧义时拒绝删除并报告原因。

## Consequences

### Positive

- 订阅列表、CPA 活动池和删除行为保持一致。
- CPA 重启或运行目录丢失后可以从归档恢复。
- Provider 扩展不需要复制 Codex 专用逻辑。

### Negative

- 运行目录中的手工添加账号会在重建时被视为孤儿并清理，必须先导入为订阅。
- 多 Provider 的额度字段不完全一致，非 Codex 账号只能展示 CPA 状态。

## Alternatives Considered

- 继续维护两套目录并做定时双向同步：容易产生冲突和令牌副本，不采用。
- 只把 75 个现有运行文件复制到 `k12/`：只能解决一次性数量问题，不能解决后续生命周期同步，不采用。

# ADR 0003: Console pagination, Luban balance, theme, and dual price history

## Status

Accepted — 2026-07-18

## Context

The console needs predictable table density, a compact service indicator, a dark appearance, Luban SMS balance visibility, and comparable K12/GPT Plus price trends without synthetic samples.

## Decision

- Full data tables use a shared pagination control with 10 rows per page. Small dashboard summary tables remain intentionally capped summaries.
- The frontend stores only the selected color theme in local storage. The backend status is represented by an accessible server icon, and the page scrollbar is visually hidden without disabling scrolling.
- The Luban API key is persisted only in backend settings. Public settings expose only a configured flag. The backend calls `https://lubansms.com/v2/api/getBalance`, and the GPT Plus page queries the balance whenever it loads.
- K12 and GPT Plus record their real average price after each successful refresh. Both histories retain 14 days and share one time axis in the dashboard chart. Failed refreshes do not create invented or zero-valued samples.

## Consequences

- Pagination behavior and page size are consistent across offers, subscriptions, alerts, and chart records.
- Browser clients never receive the saved Luban key, but the local settings file remains sensitive and must stay outside source control.
- A failed Luban request does not block GPT Plus offers; it appears as a balance error while the last offer snapshot remains available.
- Sparse history stays sparse and truthful. Short-period views may contain few points until enough real refreshes have occurred.

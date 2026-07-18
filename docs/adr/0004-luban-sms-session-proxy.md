# ADR 0004: Luban SMS session proxy

## Status

Accepted — 2026-07-18

## Context

The GPT Plus console already stored a Luban API key and queried balance. Operators also need to inspect country/service prices and manage one temporary verification-number session without exposing the key to the browser.

## Decision

- The Go API proxies only the required Luban endpoints: `countries`, `List`, `getNumber`, `getSms`, and `setStatus`.
- The browser owns the active request identifier in memory. It polls `getSms` every three seconds while the session is waiting, stops on a received code, and can explicitly release the number with `setStatus=reject`.
- The browser never calls Luban directly and never receives the saved API key. Number acquisition remains an explicit user action after selecting a country and service; page load performs read-only balance/catalog requests only.
- The catalog uses the official service list and displays the provider and real per-request cost returned by Luban.

## Failure handling

- `wrong_status` and “not received yet” responses become a normal waiting state rather than an error toast.
- Invalid keys, unavailable catalog data, and failed number requests surface as sanitized backend errors; no query string containing the key is returned to the client.
- A page close stops the local polling timer. If a number remains active, the user can reopen the page only if they still have its request identifier; the backend does not invent an untracked order state.

# ADR 0002: Dual price feeds and full-content subscription identity

## Status

Accepted

## Context

The monitor now tracks both the existing K12 offer page and the PriceAI ChatGPT Plus page. Subscription files can share an account ID while carrying different credentials or other JSON state; treating the account ID as globally unique would discard valid K12 archives and overwrite their runtime projections.

## Decision

- K12 and GPT Plus use independent scraper clients and snapshots, but one Monitor refresh schedule and one manual refresh operation.
- GPT Plus stores its latest successful snapshot in `data/gpt_plus_offers.json` and exposes it through `/api/gpt-plus`.
- A subscription is a duplicate only when its canonicalized complete JSON document matches an existing archive. Formatting and object key order do not affect equality; any JSON value difference creates a distinct archive.
- CPA runtime projection matches complete canonical JSON content, not account ID or email. Distinct credentials therefore receive distinct runtime filenames, while reconciliation removes files that are not represented by an archive.
- Loading the subscription list automatically runs the existing per-subscription connectivity/usage check for the visible page.

## Consequences

- The same account ID may produce an ambiguous CPA match if its runtime files do not expose distinct emails; the matcher prefers the subscription email before reporting ambiguity.
- GPT Plus retains its last successful data when the external page is temporarily unavailable and reports the scrape error alongside the stale snapshot.
- Subscription list loads can take longer because the visible page is checked before the refreshed rows are shown.

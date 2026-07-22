---
title: Deployment
description: Build and serve the CPA Orbit documentation as static files.
---

# Deployment

The documentation build is static. It needs no long-running Node.js process, database, or connection to a local CPA Orbit API.

## Build

```powershell
cd docs
npm ci
npm run build
```

Publish everything under:

```text
docs/.vitepress/dist/
```

## Current server path

This site is built with the base path `/cpa_orbit/` and is available at **[CPA Orbit Online Documentation](http://165.154.205.54/cpa_orbit/)**.

The matching VitePress setting is:

```ts
base: '/cpa_orbit/',
```

## Nginx

```nginx
location = /cpa_orbit {
    return 301 /cpa_orbit/;
}

location ^~ /cpa_orbit/ {
    root /;
    try_files $uri $uri.html $uri/index.html =404;
}
```

## Automatic publication

`docs/` is the single source for the online site, including [`roadmap.md`](./roadmap.md), which builds to `/cpa_orbit/roadmap`. The `Documentation` GitHub Actions workflow builds every documentation pull request and deploys documentation changes merged into `main`.

Configure a GitHub environment named `documentation` with:

| Name | Type | Purpose |
| --- | --- | --- |
| `DOCS_SSH_PRIVATE_KEY` | Secret | Private key for the restricted deployment account. |
| `DOCS_SSH_KNOWN_HOSTS` | Secret | Pinned SSH host-key entry for the documentation server. |
| `DOCS_HOST` | Variable | Deployment host, currently `165.154.205.54`. |
| `DOCS_SSH_USER` | Variable | Restricted SSH deployment user. |
| `DOCS_SSH_PORT` | Variable | SSH port; defaults to `22` when omitted. |
| `DOCS_DEPLOY_PATH` | Variable | Must be `/cpa_orbit`; the workflow refuses any other path. |

The deployment account must have write access only to `/cpa_orbit`. The workflow uploads generated `dist/` files with `rsync`, removes stale generated files within that bounded directory, and then verifies the home, English/Chinese roadmap, favicon, and commit revision marker over HTTP. Use the workflow's manual dispatch to republish the current `main` documentation without a source change.

## Release checks

- Direct refresh works on the home, guide, roadmap, and modules routes.
- English is the default and the language menu opens the matching Chinese route.
- `/cpa_orbit/favicon.svg`, `/cpa_orbit/assets/`, and `/cpa_orbit/revision.txt` return 200.
- The deployed `revision.txt` matches the commit built by GitHub Actions.
- Only the generated `dist/` contents are public.
- Source, `.git/`, `node_modules/`, and local runtime data remain outside the web surface.

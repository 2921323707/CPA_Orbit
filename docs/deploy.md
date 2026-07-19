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

## Release checks

- Direct refresh works on the home, guide, and modules routes.
- English is the default and the language menu opens the matching Chinese route.
- `/cpa_orbit/favicon.svg` and `/cpa_orbit/assets/` return 200.
- Only the generated `dist/` contents are public.
- Source, `.git/`, `node_modules/`, and local runtime data remain outside the web surface.

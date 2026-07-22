---
layout: doc
pageClass: orbit-doc-home
title: CPA Orbit 文档
---

<section class="orbit-landing-hero orbit-landing-hero--compact">
  <div class="orbit-landing-copy">
    <p class="orbit-kicker">CPA ORBIT · DOCUMENTATION</p>
    <h1>本地运行，<br>一处掌控。</h1>
    <p class="orbit-landing-tagline">把 Provider/日期订阅归档、价格信号、独立账号健康检查和 CPA/Sub2API companion 配置放进同一个本地工作台。</p>
    <div class="orbit-landing-actions">
      <a class="orbit-action orbit-action-primary" href="./guide/getting-started">快速开始</a>
      <a class="orbit-action" href="./modules/">浏览模块</a>
    </div>
  </div>
  <div class="orbit-reactor" aria-hidden="true">
    <span class="orbit-reactor__halo"></span>
    <span class="orbit-reactor__sweep"></span>
    <i class="orbit-reactor__core"></i>
    <b>LOCAL // 01</b>
    <em>ONLINE</em>
  </div>
</section>

<div class="orbit-stat-grid" aria-label="项目摘要">
  <div><span>网络</span><strong>默认仅限本机</strong></div>
  <div><span>状态</span><strong>唯一事实源</strong></div>
  <div><span>历史</span><strong>14 天价格视图</strong></div>
  <div><span>客户端</span><strong>浏览器 + 桌面端</strong></div>
</div>

## CPA Orbit 统一管理什么

<div class="orbit-feature-grid">
  <a href="./modules/#订阅资产"><span>01 / ARCHIVE</span><strong>订阅资产</strong><p>Auth JSON 安全预检进入 Provider/日期归档，并显式选择唯一兼容目标。</p></a>
  <a href="./modules/#价格情报"><span>02 / SIGNAL</span><strong>价格情报</strong><p>K12 与 GPT Plus 快照、阈值提醒和真实采集历史。</p></a>
  <a href="./SECURITY"><span>03 / LOCAL</span><strong>清晰边界</strong><p>密钥与 Token 留在后端，浏览器只接收脱敏状态。</p></a>
  <a href="./architecture/"><span>04 / RUNTIME</span><strong>共享内核</strong><p>浏览器与 Wails 桌面端复用同一个 Go 控制平面。</p></a>
</div>

## 几分钟完成启动

<div class="orbit-terminal"><pre><em>git</em> clone https://github.com/2921323707/CPA_Orbit.git
<em>cd</em> CPA_Orbit
<em>.\start-dev.ps1</em>
# 控制台  →  http://127.0.0.1:5173
# API     →  http://127.0.0.1:8090/api</pre></div>

继续阅读[快速开始](./guide/getting-started)，或打开[系统架构](./architecture/)了解数据所有权与信任边界。

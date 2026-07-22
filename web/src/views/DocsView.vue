<script setup lang="ts">
import {
  AlertTriangle,
  BellRing,
  BookOpenText,
  CheckCircle2,
  ChevronRight,
  FileJson2,
  FolderSync,
  History,
  KeyRound,
  Languages,
  PlayCircle,
  Server,
  ShieldCheck,
  ShoppingCart,
  Wrench,
} from 'lucide-vue-next'
import { ref } from 'vue'
import ReleaseChannelCard from '../components/docs/ReleaseChannelCard.vue'
import { scrollToSection } from '../utils/scrollToSection'

const changelogLanguage = ref<'zh' | 'en'>('zh')

const sections = [
  { id: 'quick-start', label: '快速开始' },
  { id: 'architecture', label: '系统组成' },
  { id: 'offers', label: 'Price 报价' },
  { id: 'gpt-plus', label: '工具箱' },
	{ id: 'subscriptions', label: '订阅与号池' },
  { id: 'alerts', label: '提醒方式' },
  { id: 'security', label: '安全边界' },
  { id: 'troubleshooting', label: '故障排查' },
]
</script>

<template>
  <div class="docs-layout">
    <div class="docs-rail">
      <aside class="docs-toc panel" aria-label="说明文档目录">
        <div class="docs-toc__title"><BookOpenText :size="18" />使用说明</div>
        <a v-for="section in sections" :key="section.id" :href="`#${section.id}`" @click.prevent="scrollToSection(section.id)">{{ section.label }}<ChevronRight :size="14" /></a>
      </aside>
      <ReleaseChannelCard />
    </div>

    <article class="docs-content page-stack">
      <section class="docs-hero panel">
        <div class="docs-hero__icon"><BookOpenText :size="28" /></div>
        <div>
			<p class="docs-eyebrow">CPA ORBIT · v1.3.0</p>
          <h1>CPA Orbit 控制台说明</h1>
			<p>从低价发现、Provider/日期订阅归档，到 CPA/Sub2API companion 配置与账号状态检查的完整操作手册。</p>
        </div>
        <div class="docs-hero__actions">
          <RouterLink class="button button--primary" to="/subscriptions">导入订阅</RouterLink>
          <RouterLink class="button button--secondary" to="/settings">打开设置</RouterLink>
        </div>
      </section>

      <section id="changelog" class="panel docs-section docs-changelog">
        <div class="docs-section__heading docs-section__heading--split">
			<div class="docs-section__heading-main"><History :size="21" /><div><h2>{{ changelogLanguage === 'zh' ? 'v1.3.0 更新日志' : 'v1.3.0 Release Notes' }}</h2><p>{{ changelogLanguage === 'zh' ? '2026-07-22 · 安全预检、显式网关分配与独立账号轮询。' : '2026-07-22 · Safe preflight, explicit gateway assignment, and independent account polling.' }}</p></div></div>
          <div class="docs-language-switch" role="group" aria-label="Changelog language">
            <Languages :size="14" />
            <button type="button" :class="{ 'is-active': changelogLanguage === 'zh' }" @click="changelogLanguage = 'zh'">中文</button>
            <button type="button" :class="{ 'is-active': changelogLanguage === 'en' }" @click="changelogLanguage = 'en'">EN</button>
          </div>
        </div>

        <div v-if="changelogLanguage === 'zh'" class="release-notes">
			<article><span>01</span><div><strong>安全 Auth JSON 预检</strong><p>先在本地验证结构、兼容性与归档目标，再确认是否执行远端写入。</p></div></article>
			<article><span>02</span><div><strong>显式唯一分配</strong><p>每份逻辑凭据只选择一个兼容 CPA 或 Sub2API 目标，不自动兜底。</p></div></article>
			<article><span>03</span><div><strong>设置中的网关配置</strong><p>网关入口移至“设置 → 网关”，不再提供独立运维路由。</p></div></article>
			<article><span>04</span><div><strong>待确认状态</strong><p>远端结果 pending 或 uncertain 时保留待核对状态，不切换另一个目标。</p></div></article>
			<article><span>05</span><div><strong>安全归属</strong><p>Orbit 托管账号可安全清理，外部接管账号只解绑、不误删远端。</p></div></article>
			<article><span>06</span><div><strong>Token 留存</strong><p>按 15 分钟保存请求、Token、延迟和成本聚合，最长保留 90 天。</p></div></article>
        </div>
        <div v-else class="release-notes">
			<article><span>01</span><div><strong>Safe Auth JSON preflight</strong><p>Validates shape, compatibility, and archive intent locally before any remote write.</p></div></article>
			<article><span>02</span><div><strong>Explicit single assignment</strong><p>Assigns each logical credential to exactly one compatible CPA or Sub2API target, with no automatic fallback.</p></div></article>
			<article><span>03</span><div><strong>Gateway settings</strong><p>Moves gateway configuration to Settings and removes the dedicated operations route.</p></div></article>
			<article><span>04</span><div><strong>Pending outcomes</strong><p>Keeps pending or uncertain remote results visible for reconciliation instead of switching targets.</p></div></article>
			<article><span>05</span><div><strong>Explicit ownership</strong><p>Managed accounts can be removed safely; adopted remote accounts are only unbound.</p></div></article>
			<article><span>06</span><div><strong>Bounded retention</strong><p>Stores 15-minute request, Token, latency, and cost aggregates for up to 90 days.</p></div></article>
        </div>
      </section>

      <section id="quick-start" class="panel docs-section">
        <div class="docs-section__heading"><PlayCircle :size="21" /><div><h2>快速开始</h2><p>建议首次使用按以下顺序操作。</p></div></div>
        <ol class="docs-steps">
          <li><span>1</span><div><strong>启动整套服务</strong><p>在项目根目录执行 <code>.\start-dev.ps1</code>。脚本会启动成熟版 CPA、Go 监控 API 和 Vue 控制台。</p></div></li>
          <li><span>2</span><div><strong>确认服务在线</strong><p>顶部服务器图标应显示在线状态；设置页中的默认 <code>base_url</code> 为 <code>http://127.0.0.1:8317/v1</code>。</p></div></li>
			<li><span>3</span><div><strong>配置网关 companion</strong><p>打开 <code>/settings?section=gateways</code>，配置本地 CPA 或 Sub2API companion。密钥只写不回显，远程地址需显式允许并使用 HTTPS。</p></div></li>
			<li><span>4</span><div><strong>预检并显式部署</strong><p>选择 Auth JSON，先完成本地安全预检与 Provider/日期归档，再明确选择唯一兼容 CPA 或 Sub2API 目标；系统不会自动兜底。</p></div></li>
          <li><span>5</span><div><strong>配置账号检查</strong><p>账号状态/额度轮询与报价监控独立，默认每 5 分钟执行；周期设为 <code>0</code> 时关闭，也可手动检查。</p></div></li>
        </ol>
      </section>

      <section id="architecture" class="panel docs-section">
        <div class="docs-section__heading"><Server :size="21" /><div><h2>系统组成</h2><p>三个本地服务协同工作，默认仅监听回环地址。</p></div></div>
        <div class="docs-service-grid">
          <article><span class="docs-service-grid__icon"><Server :size="20" /></span><strong>CLIProxyAPI 7.2.71</strong><code>127.0.0.1:8317</code><p>成熟版 CPA 服务，运行副本由订阅归档自动投影，并提供 OpenAI 兼容接口。</p></article>
          <article><span class="docs-service-grid__icon"><Wrench :size="20" /></span><strong>Monitor API</strong><code>127.0.0.1:8090</code><p>负责报价抓取、归档、提醒、配置和连通性状态，与 Sub2API 的 8080 端口隔离。</p></article>
          <article><span class="docs-service-grid__icon"><BookOpenText :size="20" /></span><strong>Vue 控制台</strong><code>127.0.0.1:5173</code><p>提供总览、报价、订阅、提醒、设置和本文档入口。</p></article>
        </div>
        <details class="docs-disclosure">
          <summary>查看项目目录说明</summary>
          <pre>CPA_monitor/
├─ cpa/                 成熟版 CLIProxyAPI 服务与配置
│  ├─ app/              可执行程序和官方文档
│  ├─ auths/            CPA 热加载认证目录
│  ├─ app/config.yaml   本地运行配置（敏感）
│  └─ logs/             服务日志
├─ server/              Go 监控 API
├─ web/                 Vue 控制台
├─ subscriptions/
│  ├─ sub2api/MMDD/      Sub2API Provider/日期归档
│  └─ cpa/MMDD/          CPA-only 归档
├─ data/                报价、提醒、设置和检查结果
└─ start-dev.ps1        Windows 一键启动</pre>
        </details>
      </section>

      <section id="offers" class="panel docs-section">
        <div class="docs-section__heading"><ShoppingCart :size="21" /><div><h2>Price 报价与购买跳转</h2><p>统一展示 K12 与 GPT Plus 未接码报价；控制台只聚合和跳转，不参与收款。</p></div></div>
        <div class="docs-callout docs-callout--info"><CheckCircle2 :size="18" /><p>系统从 PriceAI 的 ChatGPT Team / Business 页面筛选链动小铺 K12 商品并保留最低 5 条，同时展示 ChatGPT Plus 未接码账号；两个报价源使用同一刷新周期。</p></div>
        <ul class="docs-list">
          <li>“刷新报价”会立即重新抓取，后台也会按设置周期自动刷新。</li>
          <li>链动小铺 ID 与商品 itemId 是两个不同字段；详情抽屉会同时展示。</li>
          <li>低于阈值的记录会显示图标和“低于阈值”文字，不只依赖颜色。</li>
          <li>“购买/直达支付”在新标签页打开链动小铺；付款和售后仍由对应商家负责。</li>
        </ul>
      </section>

      <section id="gpt-plus" class="panel docs-section">
        <div class="docs-section__heading"><ShoppingCart :size="21" /><div><h2>工具箱与接码</h2><p>GPT Plus 报价统一显示在 Price 页面；外部 JSON 转换器已移除，工具箱保留鲁班接码。</p></div></div>
        <ul class="docs-list">
          <li>GPT Plus 来源页面为 <code>https://priceai.cc/products/chatgpt-plus?tags=account_unverified</code>，只采集未接码账号，并与 K12 共用后台刷新周期。</li>
          <li>Price 页面按价格升序展示当前未接码账号快照；抓取失败时保留旧数据并显示错误原因。</li>
          <li>“刷新报价”会同时刷新 K12 和 GPT Plus 两个报价源。</li>
          <li>报价表固定每页 5 条；总览的平均报价走势可在 K12 与 GPT Plus 之间切换，并按真实查询时间保留最近 14 天记录。</li>
          <li>外部订阅 JSON 转换器已移除；敏感 Auth JSON 直接在订阅页执行本地安全预检。</li>
          <li>“鲁班接码”可保存 API 密钥；进入页面时会自动查询余额，并加载国家、服务和单次价格。密钥只保存在后端，本页只显示配置状态和余额。</li>
          <li>选择国家与服务后，点击“获取号码”才会创建真实接码订单；号码需要手动填写到目标平台并发送验证码，控制台每 3 秒检查一次短信，收到后可复制验证码或释放号码。</li>
        </ul>
      </section>

      <section id="subscriptions" class="panel docs-section">
		<div class="docs-section__heading"><FileJson2 :size="21" /><div><h2>订阅文件与显式网关分配</h2><p>Provider/日期归档、运行账号和检查状态分层管理，每份逻辑凭据只分配给一个活动池。</p></div></div>
        <div class="docs-flow">
			<span><FileJson2 :size="18" />安全预检</span><ChevronRight :size="17" /><span><FolderSync :size="18" />Provider/日期归档</span><ChevronRight :size="17" /><span><Server :size="18" />选择唯一目标</span><ChevronRight :size="17" /><span><CheckCircle2 :size="18" />确认或待核对</span>
        </div>
        <details class="docs-disclosure" open>
          <summary>字段和状态说明</summary>
          <div class="docs-table-wrap"><table><thead><tr><th>字段</th><th>含义</th></tr></thead><tbody>
            <tr><td>邮箱</td><td>从 JSON 的 <code>email</code> 或 <code>name</code> 读取。</td></tr>
			<tr><td>运行池</td><td>显示逻辑凭据唯一的 CPA 或 Sub2API 分配；没有绑定、pending 或 uncertain 时仍保留本地归档与待核对状态。</td></tr>
            <tr><td>延迟</td><td>逐账号请求 ChatGPT usage 接口的完整往返耗时；未入 CPA 池的归档不显示虚假的 0 ms。</td></tr>
            <tr><td>5H / 7D 额度</td><td>显示上游返回窗口的剩余百分比和重置时间；账号未提供对应窗口时显示“—”。</td></tr>
            <tr><td>有效期</td><td>按 JSON 的 <code>expired</code> 计算文件剩余有效天数，与 5H/7D 使用额度相互独立。</td></tr>
            <tr><td>订阅文件</td><td>日期归档中的原始文件名，不显示 token；只有规范化后的完整 JSON 内容完全一致才判定为重复。</td></tr>
            <tr><td>入手价格</td><td>单个导入时可填写的本地记录项；不填写也可以继续，批量导入不设置该项。</td></tr>
            <tr><td>计划轮询</td><td>账号状态/额度与报价监控使用独立周期，默认每 5 分钟；设为 <code>0</code> 时关闭，不由页面加载触发。</td></tr>
          </tbody></table></div>
        </details>
		<div class="docs-callout docs-callout--warning"><AlertTriangle :size="18" /><p>同一个可刷新的 OAuth 凭据只能显式分配给一个兼容 CPA 或 Sub2API 目标。远端失败或结果不确定时不会自动兜底；请先核对 pending/uncertain 状态再重试。订阅转网关可能与上游服务条款冲突，请只在获得授权时使用。</p></div>
      </section>

      <section id="alerts" class="panel docs-section">
        <div class="docs-section__heading"><BellRing :size="21" /><div><h2>提醒方式</h2><p>提醒中心已集成到 Settings 子导航；后端历史与浏览器本地偏好相互独立。</p></div></div>
        <div class="docs-feature-grid">
          <article><strong>界面提醒</strong><p>低价横幅、状态标签、Toast 和历史提醒；历史最多保留 10 条，每页显示 5 条。</p></article>
          <article><strong>浏览器通知</strong><p>需要浏览器授权；页面打开时可以显示系统通知。</p></article>
          <article><strong>提示声音</strong><p>通过浏览器 Web Audio 播放短音，不上传任何数据。</p></article>
          <article><strong>Webhook</strong><p>由 Go 后端向设置的 URL 发送 JSON，适合企业微信、钉钉或自建服务。</p></article>
        </div>
      </section>

      <section id="security" class="panel docs-section">
        <div class="docs-section__heading"><ShieldCheck :size="21" /><div><h2>安全边界</h2><p>CPA JSON 包含 bearer token，应按密码文件处理。</p></div></div>
        <ul class="docs-list docs-list--check">
          <li>CPA、Go API 和前端默认只绑定 <code>127.0.0.1</code>。</li>
			<li>前端 API 永不返回 access token、refresh token、id token、Sub2API 管理密钥、已保存的 CPA API key 或鲁班 API key；所有网关密钥只写不回显。</li>
          <li><code>subscriptions/**/*.json</code>、<code>cpa/auths/**</code>、本地配置和运行数据均应保持在 Git 之外；备份需包含 Provider/日期归档与 <code>data/</code>，并加密和限制访问。</li>
			<li>默认只允许检查回环 base_url；远程网关必须显式开启并使用 HTTPS。</li>
          <li>不要通过聊天、截图、网盘或公开仓库分享 Auth JSON。本地凭据和设置可能明文落盘，除非主机文件系统提供加密。</li>
        </ul>
      </section>

      <section id="troubleshooting" class="panel docs-section">
        <div class="docs-section__heading"><Wrench :size="21" /><div><h2>故障排查</h2><p>先看状态，再按错误类型处理。</p></div></div>
        <div class="docs-faq">
          <details open><summary>顶部服务器图标显示离线</summary><p>确认 <code>start-dev.ps1</code> 窗口仍在运行，并访问 <code>http://127.0.0.1:8090/api/health</code>。端口被占用时先关闭旧实例。</p></details>
          <details><summary>账号检测返回 HTTP 401</summary><p>这表示当前文件对应的 OAuth access token 无效、过期或刷新失败。优先重新获取该账号的 CPA JSON，而不是修改控制台客户端 API key。</p></details>
          <details><summary>连通性测试返回 HTTP 402</summary><p>代理已经收到请求，但上游账号通常不可用、额度不足或订阅失效。打开订阅详情确认文件和到期日，必要时重新购买并导入新 JSON。</p></details>
          <details><summary>导入后 CPA 没有加载</summary><p>确认设置中的 CPA auth-dir 指向项目 <code>cpa/auths</code>，并开启“导入后同步到 CPA”。也可在订阅详情中手动点击“同步到 CPA”。</p></details>
          <details><summary>报价没有更新</summary><p>检查网络能否访问 PriceAI，再手动点击“刷新报价”。旧快照会保留，避免上游短暂失败导致页面清空。</p></details>
        </div>
      </section>

      <footer class="docs-footer">
		<KeyRound :size="16" />CPA Orbit v1.3.0 · 本文档不会展示任何密钥；敏感配置请只在设置页填写。
      </footer>
    </article>
  </div>
</template>

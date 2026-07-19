import { defineConfig } from 'vitepress'

const englishSidebar = [
  {
    text: 'Start',
    items: [
      { text: 'Introduction', link: '/' },
      { text: 'Quick start', link: '/guide/getting-started' },
      { text: 'Deployment', link: '/deploy' },
    ],
  },
  {
    text: 'Product',
    items: [
      { text: 'Modules', link: '/modules/' },
      { text: 'Architecture', link: '/architecture/' },
      { text: 'Roadmap', link: '/roadmap' },
    ],
  },
  {
    text: 'Developers',
    items: [
      { text: 'Backend', link: '/development/backend' },
      { text: 'Desktop', link: '/development/desktop' },
      { text: 'Architecture decisions', link: '/architecture/adr/' },
      { text: 'Changelog', link: '/releases/CHANGELOG' },
    ],
  },
  {
    text: 'Community',
    items: [
      { text: 'Contributing', link: '/contribute' },
      { text: 'Sponsor', link: '/sponsor' },
      { text: 'Security', link: '/SECURITY' },
      { text: 'Support', link: '/SUPPORT' },
    ],
  },
]

const chineseSidebar = [
  {
    text: '开始',
    items: [
      { text: '项目简介', link: '/zh/' },
      { text: '快速开始', link: '/zh/guide/getting-started' },
      { text: '部署上线', link: '/zh/deploy' },
    ],
  },
  {
    text: '产品',
    items: [
      { text: '模块介绍', link: '/zh/modules/' },
      { text: '系统架构', link: '/zh/architecture/' },
      { text: '后期扩展', link: '/zh/roadmap' },
    ],
  },
  {
    text: '开发者',
    items: [
      { text: '后端开发', link: '/zh/development/backend' },
      { text: '桌面端开发', link: '/zh/development/desktop' },
      { text: '架构决策', link: '/zh/architecture/adr/' },
      { text: '更新日志', link: '/zh/releases/CHANGELOG' },
    ],
  },
  {
    text: '社区',
    items: [
      { text: '参与贡献', link: '/zh/contribute' },
      { text: '赞助项目', link: '/zh/sponsor' },
      { text: '安全策略', link: '/zh/SECURITY' },
      { text: '支持与反馈', link: '/zh/SUPPORT' },
    ],
  },
]

export default defineConfig({
  base: '/cpa_orbit/',
  cleanUrls: true,
  lastUpdated: true,
  appearance: true,
  rewrites: {
    'architecture/README.md': 'architecture/index.md',
  },
  head: [
    ['meta', { name: 'theme-color', content: '#0b6159' }],
    ['meta', { name: 'color-scheme', content: 'light dark' }],
    ['link', { rel: 'icon', href: '/cpa_orbit/favicon.svg', type: 'image/svg+xml' }],
  ],
  locales: {
    root: {
      label: 'English',
      lang: 'en-US',
      title: 'CPA Orbit',
      description: 'Local-first operations for AI subscriptions, price intelligence, quota health, and CPA runtime.',
      themeConfig: {
        nav: [
          { text: 'Guide', link: '/guide/getting-started' },
          { text: 'Modules', link: '/modules/' },
          { text: 'Roadmap', link: '/roadmap' },
          { text: 'Contribute', link: '/contribute' },
          {
            text: 'v1.1.0',
            items: [
              { text: 'Deploy docs', link: '/deploy' },
              { text: 'Changelog', link: '/releases/CHANGELOG' },
              { text: 'GitHub', link: 'https://github.com/2921323707/CPA_Orbit' },
            ],
          },
        ],
        sidebar: englishSidebar,
        outline: { label: 'On this page', level: [2, 3] },
        lastUpdated: { text: 'Last updated' },
        docFooter: { prev: 'Previous', next: 'Next' },
        returnToTopLabel: 'Return to top',
        sidebarMenuLabel: 'Documentation',
        darkModeSwitchLabel: 'Appearance',
        lightModeSwitchTitle: 'Switch to light theme',
        darkModeSwitchTitle: 'Switch to dark theme',
        footer: {
          message: 'Local first. Boundaries explicit.',
          copyright: 'Released under the MIT License.',
        },
      },
    },
    zh: {
      label: '简体中文',
      lang: 'zh-CN',
      link: '/zh/',
      title: 'CPA Orbit',
      description: '本地优先的 AI 订阅、价格情报、额度健康与 CPA 运行控制台。',
      themeConfig: {
        nav: [
          { text: '指南', link: '/zh/guide/getting-started' },
          { text: '模块', link: '/zh/modules/' },
          { text: '扩展', link: '/zh/roadmap' },
          { text: '参与', link: '/zh/contribute' },
          {
            text: 'v1.1.0',
            items: [
              { text: '部署文档站', link: '/zh/deploy' },
              { text: '更新日志', link: '/zh/releases/CHANGELOG' },
              { text: 'GitHub', link: 'https://github.com/2921323707/CPA_Orbit' },
            ],
          },
        ],
        sidebar: chineseSidebar,
        outline: { label: '本页目录', level: [2, 3] },
        lastUpdated: { text: '最后更新于' },
        docFooter: { prev: '上一页', next: '下一页' },
        returnToTopLabel: '返回顶部',
        sidebarMenuLabel: '文档导航',
        darkModeSwitchLabel: '切换主题',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式',
        footer: {
          message: '本地优先，边界清晰。',
          copyright: '基于 MIT 许可证发布。',
        },
      },
    },
  },
  themeConfig: {
    logo: '/favicon.svg',
    siteTitle: 'CPA Orbit',
    i18nRouting(_data, route, targetLocale) {
      let path = route.data.relativePath
        .replace(/^zh\//, '')
        .replace(/\.md$/, '')
        .replace(/README$/, '')
        .replace(/index$/, '')

      if (targetLocale === 'zh') {
        if (/^architecture\/adr\/000/.test(path)) path = 'architecture/adr/'
        if (/^(development\/plans|plans)\//.test(path)) path = 'roadmap'
        if (/^(CONTRIBUTING|CODE_OF_CONDUCT|community\/)/.test(path)) path = 'contribute'
        if (/^(THIRD_PARTY_NOTICES|README)$/.test(path)) path = ''
        if (path === 'releases/v1.0.2') path = 'releases/CHANGELOG'
        return `/zh/${path}`
      }

      return `/${path}`
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/2921323707/CPA_Orbit' },
    ],
    search: { provider: 'local' },
  },
})

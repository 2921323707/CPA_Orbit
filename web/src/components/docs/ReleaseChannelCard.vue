<script setup lang="ts">
import { ChevronRight, Github, Radio } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { getGitHubRepository, PROJECT_GITHUB_URL, PROJECT_VERSION } from '../../constants/project'
import { useLocale } from '../../i18n'
import { scrollToSection } from '../../utils/scrollToSection'

type GitHubRelease = {
  html_url: string
  name: string | null
  published_at: string | null
  tag_name: string
}

const { isEnglish, t } = useLocale()
const latestRelease = ref<GitHubRelease | null>(null)
const syncState = ref<'local' | 'syncing' | 'synced' | 'fallback'>('local')
const repository = getGitHubRepository()

const versionLabel = computed(() => latestRelease.value?.tag_name || `v${PROJECT_VERSION}`)
const releaseDate = computed(() => {
  const publishedAt = latestRelease.value?.published_at
  if (!publishedAt) return '2026-07-18'
  return new Intl.DateTimeFormat(isEnglish.value ? 'en-US' : 'zh-CN', { dateStyle: 'medium' }).format(new Date(publishedAt))
})
const syncLabel = computed(() => {
  if (syncState.value === 'syncing') return t('docs.releaseSyncing')
  if (syncState.value === 'synced') return t('docs.releaseSynced')
  if (syncState.value === 'fallback') return t('docs.releaseFallback')
  return t('docs.releasePending')
})

onMounted(async () => {
  if (!repository) return
  syncState.value = 'syncing'
  try {
    const response = await fetch(`https://api.github.com/repos/${repository}/releases/latest`, {
      headers: { Accept: 'application/vnd.github+json' },
    })
    if (!response.ok) throw new Error(`GitHub release request failed: ${response.status}`)
    latestRelease.value = await response.json() as GitHubRelease
    syncState.value = 'synced'
  } catch {
    syncState.value = 'fallback'
  }
})
</script>

<template>
  <section class="docs-release-channel panel" :aria-label="t('docs.releaseChannel')">
    <div class="docs-release-channel__top">
      <span class="docs-release-channel__icon"><Radio :size="15" /></span>
      <span>{{ t('docs.releaseChannel') }}</span>
      <i :class="{ 'is-live': syncState === 'synced' }" />
    </div>
    <div class="docs-release-channel__version">
      <strong>{{ versionLabel }}</strong>
      <span>{{ releaseDate }}</span>
    </div>
    <a class="docs-release-channel__link" href="#changelog" @click.prevent="scrollToSection('changelog')">
      {{ t('docs.release') }}<ChevronRight :size="14" />
    </a>
    <a
      v-if="latestRelease"
      class="docs-release-channel__source"
      :href="latestRelease.html_url"
      target="_blank"
      rel="noopener noreferrer"
    >
      <Github :size="12" />{{ latestRelease.name || latestRelease.tag_name }}
    </a>
    <p v-else class="docs-release-channel__source" :title="PROJECT_GITHUB_URL || t('shell.githubPending')">
      <Github :size="12" />{{ syncLabel }}
    </p>
  </section>
</template>

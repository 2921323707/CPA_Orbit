import { expect, test } from '@playwright/test'

test('docs in-page links do not replace the desktop hash route', async ({ page }) => {
  await page.route('**/api/**', async (route) => {
    await route.fulfill({ status: 404, json: { message: 'Not needed for documentation navigation' } })
  })
  await page.route('https://api.github.com/**', async (route) => {
    await route.fulfill({ status: 503, json: { message: 'Offline in navigation test' } })
  })

  await page.goto('/#/docs')
  await expect(page).toHaveTitle(/^(使用说明|Docs) · CPA Orbit$/)

  const sectionIds = [
    'quick-start',
    'architecture',
    'offers',
    'gpt-plus',
    'subscriptions',
    'alerts',
    'security',
    'troubleshooting',
  ]
  for (const sectionId of sectionIds) {
    await page.locator(`.docs-toc a[href="#${sectionId}"]`).click()
    await expect(page).toHaveURL('http://127.0.0.1:4174/#/docs')
    await expect(page.locator(`#${sectionId}`)).toBeInViewport()
  }

  await page.locator('.docs-release-channel__link').click()
  await expect(page).toHaveURL('http://127.0.0.1:4174/#/docs')
  await expect(page.getByRole('heading', { name: /更新日志|Release Notes/ })).toBeInViewport()
})

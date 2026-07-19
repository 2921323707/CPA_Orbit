export const PROJECT_NAME = 'CPA Orbit'
export const PROJECT_VERSION = '1.1.0'

// Set this when the public repository is ready. The topbar automatically turns
// the reserved GitHub mark into a link as soon as this value is non-empty.
export const PROJECT_GITHUB_URL = 'https://github.com/2921323707/CPA_Orbit'

export function getGitHubRepository() {
  const match = PROJECT_GITHUB_URL.match(/github\.com\/([^/]+)\/([^/#?]+?)(?:\.git)?\/?$/i)
  return match ? `${match[1]}/${match[2]}` : ''
}

import { ref } from 'vue'

export const navigationLoading = ref(true)
export const initialNavigation = ref(true)

let finishTimer: number | undefined
let firstNavigation = true

export function beginNavigation() {
  if (finishTimer !== undefined) window.clearTimeout(finishTimer)
  navigationLoading.value = true
}

export function finishNavigation() {
	const delay = firstNavigation ? 260 : 90
	firstNavigation = false
	finishTimer = window.setTimeout(() => {
		navigationLoading.value = false
		initialNavigation.value = false
    finishTimer = undefined
  }, delay)
}

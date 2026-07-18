import { readonly, ref } from 'vue'

export type ToastType = 'success' | 'error' | 'info'
export interface ToastItem {
  id: number
  type: ToastType
  message: string
}

const toasts = ref<ToastItem[]>([])
let nextId = 1

export function useToast() {
  function remove(id: number) {
    toasts.value = toasts.value.filter((item) => item.id !== id)
  }

  function show(message: string, type: ToastType = 'info') {
    const id = nextId++
    toasts.value.push({ id, type, message })
    window.setTimeout(() => remove(id), 3800)
  }

  return {
    toasts: readonly(toasts),
    show,
    success: (message: string) => show(message, 'success'),
    error: (message: string) => show(message, 'error'),
    remove,
  }
}

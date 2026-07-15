export type LegacyToolCleanup = () => void

export type LegacyToolServiceMap = Record<string, unknown>

export interface LegacyDocumentFacade {
  getElementById(elementId: string): HTMLElement | null
  querySelector<E extends Element = Element>(selectors: string): E | null
  querySelectorAll<E extends Element = Element>(selectors: string): NodeListOf<E>
  addEventListener(type: string, listener: EventListenerOrEventListenerObject, options?: boolean | AddEventListenerOptions): void
  removeEventListener(type: string, listener: EventListenerOrEventListenerObject, options?: boolean | EventListenerOptions): void
  readonly activeElement: Element | null
  startViewTransition?: (callback: () => void) => unknown
}

export interface LegacyWindowFacade {
  CSS: typeof window.CSS
  location: {
    readonly protocol: string
    href: string
  }
  setTimeout(handler: TimerHandler, timeout?: number, ...args: unknown[]): number
  clearTimeout(handle?: number): void
  setInterval(handler: TimerHandler, timeout?: number, ...args: unknown[]): number
  clearInterval(handle?: number): void
  requestAnimationFrame(callback: FrameRequestCallback): number
  cancelAnimationFrame(handle: number): void
  confirm(message?: string): boolean
  prompt(message?: string, defaultValue?: string): string | null
}

export interface LegacyToolScope {
  document: LegacyDocumentFacade
  window: LegacyWindowFacade
  services: LegacyToolServiceMap
  localStorage: Storage
  fetch: typeof fetch
  Headers: typeof Headers
  CSS: typeof window.CSS
  structuredClone: <T>(value: T) => T
  console: Console
  cleanup: LegacyToolCleanup
}

export interface LegacyToolScopeOptions {
  onNavigate?: (target: string) => boolean | void
  services?: LegacyToolServiceMap
}

// 为旧单页工具创建局部浏览器环境，确保 DOM 查询、定时器和全局事件都收束在当前工具实例里。
export function createLegacyToolScope(root: ShadowRoot, options: LegacyToolScopeOptions = {}): LegacyToolScope {
  const eventCleanups: LegacyToolCleanup[] = []
  const timeoutIds = new Set<number>()
  const intervalIds = new Set<number>()
  const animationFrameIds = new Set<number>()
  let scopedHref = window.location.href

  function removeDocumentListener(
    type: string,
    listener: EventListenerOrEventListenerObject,
    listenerOptions?: boolean | EventListenerOptions
  ) {
    document.removeEventListener(type, listener, listenerOptions)
  }

  const scopedDocument: LegacyDocumentFacade = {
    getElementById(elementId: string) {
      return root.getElementById(elementId) as HTMLElement | null
    },
    querySelector<E extends Element = Element>(selectors: string) {
      return root.querySelector<E>(selectors)
    },
    querySelectorAll<E extends Element = Element>(selectors: string) {
      return root.querySelectorAll<E>(selectors)
    },
    addEventListener(type: string, listener: EventListenerOrEventListenerObject, listenerOptions?: boolean | AddEventListenerOptions) {
      document.addEventListener(type, listener, listenerOptions)
      eventCleanups.push(() => removeDocumentListener(type, listener, listenerOptions))
    },
    removeEventListener(type: string, listener: EventListenerOrEventListenerObject, listenerOptions?: boolean | EventListenerOptions) {
      removeDocumentListener(type, listener, listenerOptions)
    },
    get activeElement() {
      return root.activeElement ?? document.activeElement
    },
    startViewTransition(callback: () => void) {
      const viewTransition = (document as Document & { startViewTransition?: (callback: () => void) => unknown }).startViewTransition
      if (typeof viewTransition === 'function') {
        return viewTransition.call(document, callback)
      }
      callback()
      return undefined
    }
  }

  const scopedWindow: LegacyWindowFacade = {
    CSS: window.CSS,
    location: {
      get protocol() {
        return window.location.protocol
      },
      get href() {
        return scopedHref
      },
      set href(target: string) {
        scopedHref = target
        const handled = options.onNavigate?.(target)
        if (handled === false) {
          window.location.href = target
        }
      }
    },
    setTimeout(handler: TimerHandler, timeout?: number, ...args: unknown[]) {
      const id = window.setTimeout(handler, timeout, ...args)
      timeoutIds.add(id)
      return id
    },
    clearTimeout(handle?: number) {
      if (handle !== undefined) {
        timeoutIds.delete(handle)
      }
      window.clearTimeout(handle)
    },
    setInterval(handler: TimerHandler, timeout?: number, ...args: unknown[]) {
      const id = window.setInterval(handler, timeout, ...args)
      intervalIds.add(id)
      return id
    },
    clearInterval(handle?: number) {
      if (handle !== undefined) {
        intervalIds.delete(handle)
      }
      window.clearInterval(handle)
    },
    requestAnimationFrame(callback: FrameRequestCallback) {
      const id = window.requestAnimationFrame(callback)
      animationFrameIds.add(id)
      return id
    },
    cancelAnimationFrame(handle: number) {
      animationFrameIds.delete(handle)
      window.cancelAnimationFrame(handle)
    },
    confirm(message?: string) {
      return window.confirm(message)
    },
    prompt(message?: string, defaultValue?: string) {
      return window.prompt(message, defaultValue)
    }
  }

  // 页签切换或组件卸载时统一清理旧脚本留下的后台资源，避免重复轮询和全局快捷键残留。
  function cleanup() {
    eventCleanups.splice(0).forEach(remove => remove())
    timeoutIds.forEach(id => window.clearTimeout(id))
    timeoutIds.clear()
    intervalIds.forEach(id => window.clearInterval(id))
    intervalIds.clear()
    animationFrameIds.forEach(id => window.cancelAnimationFrame(id))
    animationFrameIds.clear()
  }

  return {
    document: scopedDocument,
    window: scopedWindow,
    services: options.services ?? {},
    localStorage: window.localStorage,
    fetch: window.fetch.bind(window),
    Headers: window.Headers,
    CSS: window.CSS,
    structuredClone: typeof window.structuredClone === 'function'
      ? window.structuredClone.bind(window)
      : <T>(value: T) => JSON.parse(JSON.stringify(value)) as T,
    console,
    cleanup
  }
}

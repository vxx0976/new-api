import { useEffect, useState } from 'react'

import { getSelfAgent } from './api'

// 模块级缓存：侧边栏每次挂载都问一次没必要，一个页面生命周期问一次就够。
let cached: boolean | null = null
let inFlight: Promise<boolean> | null = null

function resolveIsAgent(): Promise<boolean> {
  if (cached !== null) return Promise.resolve(cached)
  if (!inFlight) {
    inFlight = getSelfAgent()
      .then((data) => {
        cached = Boolean(data.is_agent)
        return cached
      })
      .catch(() => false)
      .finally(() => {
        inFlight = null
      })
  }
  return inFlight
}

/** 清掉缓存，开通/注销代理之后调用。 */
export function invalidateIsAgent() {
  cached = null
}

/**
 * 当前登录用户是否拥有代理。用于决定侧边栏要不要显示「代理后台」入口——
 * 非代理看到这个入口只会点进一个"你不是代理"的空页面，属于噪音。
 */
export function useIsAgent(): boolean {
  const [isAgent, setIsAgent] = useState(cached ?? false)

  useEffect(() => {
    let alive = true
    void resolveIsAgent().then((value) => {
      if (alive) setIsAgent(value)
    })
    return () => {
      alive = false
    }
  }, [])

  return isAgent
}

import { useState, useEffect } from "react";

const MOBILE_BREAKPOINT = 768;

export function useIsMobile() {
  const [isMobile, setIsMobile] = useState<boolean | undefined>(() => {
    // 懒初始化，避免首次渲染后再同步触发一次状态更新。
    return window.innerWidth < MOBILE_BREAKPOINT;
  });

  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
    const onChange = () => {
      setIsMobile(window.innerWidth < MOBILE_BREAKPOINT);
    };
    mql.addEventListener("change", onChange);
    // 初始值已经在 useState 中计算，这里只订阅后续变化。
    return () => mql.removeEventListener("change", onChange);
  }, []);

  return !!isMobile;
}

import { useLayoutEffect } from "react";

export const MOBILE_LAYOUT_QUERY = "(max-width: 900px)";

let activeLocks = 0;
let restoreDocument: (() => void) | null = null;

/** Share one saved document position across all open mobile overlays. */
function acquireDocumentLock() {
  if (activeLocks === 0) {
    const { scrollX, scrollY } = window;
    const root = document.documentElement;
    const body = document.body;
    const scrollbarWidth = window.innerWidth - root.clientWidth;
    const paddingRight = getComputedStyle(body).paddingRight;
    const restoreStyles: (() => void)[] = [];

    function setStyle(element: HTMLElement, property: string, value: string) {
      const previous = element.style.getPropertyValue(property);
      const priority = element.style.getPropertyPriority(property);
      element.style.setProperty(property, value);
      restoreStyles.push(() => {
        if (previous) element.style.setProperty(property, previous, priority);
        else element.style.removeProperty(property);
      });
    }

    setStyle(root, "overflow-y", "hidden");
    setStyle(root, "overscroll-behavior-y", "none");
    setStyle(body, "position", "fixed");
    setStyle(body, "top", `${-scrollY}px`);
    setStyle(body, "left", `${-scrollX}px`);
    setStyle(body, "width", "100%");
    if (scrollbarWidth > 0) {
      setStyle(body, "padding-right", `calc(${paddingRight} + ${scrollbarWidth}px)`);
    }

    restoreDocument = () => {
      for (const restore of restoreStyles.reverse()) restore();
      window.scrollTo({ left: scrollX, top: scrollY, behavior: "instant" });
    };
  }

  activeLocks += 1;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    activeLocks -= 1;
    if (activeLocks === 0) {
      const restore = restoreDocument;
      restoreDocument = null;
      restore?.();
    }
  };
}

/** Freeze mobile document scrolling before an overlay moves focus. */
export function useMobileScrollLock(enabled: boolean) {
  useLayoutEffect(() => {
    if (!enabled || !window.matchMedia) return;
    const query = window.matchMedia(MOBILE_LAYOUT_QUERY);
    let release: (() => void) | null = null;
    const update = () => {
      if (query.matches && !release) {
        release = acquireDocumentLock();
      } else if (!query.matches && release) {
        release();
        release = null;
      }
    };
    update();
    query.addEventListener("change", update);
    return () => {
      query.removeEventListener("change", update);
      release?.();
    };
  }, [enabled]);
}

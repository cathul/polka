import { useEffect, useRef } from "react";

// Opening/restoring/resizing a book is not a reading-progress update.
// Permit saves after deliberate scrolling, chapter navigation (by callers),
// or a scrollbar drag; a click on Back/settings alone does not enable saves.
export const useReadingActivity = (bookId) => {
  const active = useRef(false);
  useEffect(() => {
    active.current = false;
    const mark = () => { active.current = true; };
    const key = (event) => {
      if (event.target.closest?.("input, textarea, select")) return;
      if (["ArrowUp", "ArrowDown", "PageUp", "PageDown", " ", "Home", "End"].includes(event.key)) mark();
    };
    const scrollbar = (event) => {
      if (event.target === document.documentElement && event.clientX >= document.documentElement.clientWidth) mark();
    };
    window.addEventListener("wheel", mark, { passive: true });
    window.addEventListener("touchmove", mark, { passive: true });
    window.addEventListener("keydown", key);
    window.addEventListener("pointerdown", scrollbar);
    return () => {
      window.removeEventListener("wheel", mark);
      window.removeEventListener("touchmove", mark);
      window.removeEventListener("keydown", key);
      window.removeEventListener("pointerdown", scrollbar);
    };
  }, [bookId]);
  return active;
};

// DOM offsets are UTF-16; CREngine XPointers use Unicode code points.
export const codePointOffset = (text, offset) => Array.from(text.slice(0, offset)).length;
export const domOffset = (text, offset) => Array.from(text).slice(0, offset).join("").length;

export const readingFraction = (meta, index, ratio) => {
  const chapters = meta?.chapters ?? [];
  const total = chapters.reduce((sum, chapter) => sum + (chapter.textLength ?? 0), 0);
  if (!total) return chapters.length ? (index + ratio) / chapters.length : 0;
  const before = chapters.slice(0, index).reduce((sum, chapter) => sum + (chapter.textLength ?? 0), 0);
  return (before + ratio * (chapters[index]?.textLength ?? 0)) / total;
};

// Called on the debounced save, not on every animation frame or card render.
export const captureFB2Position = (root, index, meta) => {
  if (!meta?.fb2Digest) return null;
  const content = root?.querySelector(`[data-chapter="${index}"] .reader__content`);
  if (!content) return null;
  const rect = content.getBoundingClientRect();
  const x = Math.max(0, Math.min(window.innerWidth - 1, rect.left + 24));
  const y = window.innerHeight * 0.35;
  let node, offset;
  if (document.caretPositionFromPoint) {
    const caret = document.caretPositionFromPoint(x, y);
    node = caret?.offsetNode; offset = caret?.offset;
  } else if (document.caretRangeFromPoint) {
    const caret = document.caretRangeFromPoint(x, y);
    node = caret?.startContainer; offset = caret?.startOffset;
  }
  if (!node || node.nodeType !== Node.TEXT_NODE || !content.contains(node)) return null;
  const marker = node.parentElement;
  const path = marker?.dataset.fb2Node;
  if (!path || marker.firstChild !== node || node.nextSibling || offset < 0 || offset > node.length) return null;
  const prefix = document.createRange();
  prefix.setStart(content, 0); prefix.setEnd(node, offset);
  const lengths = meta.chapters.map((chapter) => chapter.textLength ?? 0);
  const total = lengths.reduce((sum, length) => sum + length, 0);
  if (!total) return null;
  const before = lengths.slice(0, index).reduce((sum, length) => sum + length, 0);
  const progress = Math.max(0, Math.min(1, (before + Array.from(prefix.toString()).length) / total));
  return {
    locator: "fb2:" + JSON.stringify({ digest: meta.fb2Digest, path, offset: codePointOffset(node.data, offset), chapter: index }),
    progress,
  };
};

// Restore the actual text range; chapter/ratio remains the safe fallback.
export const restoreFB2Position = (root, anchor) => {
  if (!anchor) return false;
  const markers = root?.querySelectorAll("[data-fb2-node]") ?? [];
  const marker = Array.from(markers).find((element) => element.dataset.fb2Node === anchor.path);
  const node = marker?.firstChild;
  if (!node || node.nodeType !== Node.TEXT_NODE || anchor.offset > Array.from(node.data).length) return false;
  const range = document.createRange();
  const offset = domOffset(node.data, anchor.offset);
  range.setStart(node, offset);
  range.setEnd(node, Math.min(node.length, offset + (node.data.codePointAt(offset) > 0xffff ? 2 : 1)));
  if (!range.getBoundingClientRect) return false;
  const rect = range.getBoundingClientRect();
  if (!rect.height) return false;
  window.scrollTo({ top: Math.max(0, window.scrollY + rect.top - window.innerHeight * 0.35), left: 0, behavior: "instant" });
  return true;
};

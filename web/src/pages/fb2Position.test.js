import { afterEach, expect, it, vi } from "vitest";
import { captureFB2Position, codePointOffset, domOffset, restoreFB2Position } from "./fb2Position";

const originalCaret = Object.getOwnPropertyDescriptor(document, "caretPositionFromPoint");
afterEach(() => {
  if (originalCaret) Object.defineProperty(document, "caretPositionFromPoint", originalCaret);
  else delete document.caretPositionFromPoint;
  document.querySelectorAll("[data-test-reader]").forEach((element) => element.remove());
  vi.restoreAllMocks();
});

const fixture = () => {
  const root = document.createElement("div");
  root.dataset.testReader = "true";
  root.innerHTML = '<section data-chapter="0"><article class="reader__content"><p><span data-fb2-node="/FictionBook[1]/body[1]/section[1]/p[1]/text()[1]">А😀Б текст</span></p></article></section>';
  document.body.append(root);
  return root;
};

it("converts browser UTF-16 offsets and CREngine code-point offsets without splitting emoji", () => {
  expect(codePointOffset("А😀Б", 3)).toBe(2);
  expect(domOffset("А😀Б", 2)).toBe(3);
});

it("captures the actual source text location and text-weighted fraction at the reading line", () => {
  const root = fixture();
  const node = root.querySelector("span").firstChild;
  Object.defineProperty(document, "caretPositionFromPoint", { configurable: true, value: () => ({ offsetNode: node, offset: 3 }) });
  const location = captureFB2Position(root, 0, { fb2Digest: "original-fb2", chapters: [{ textLength: 9 }] });
  expect(JSON.parse(location.locator.slice(4))).toMatchObject({ digest: "original-fb2", offset: 2, chapter: 0 });
  expect(location.progress).toBeCloseTo(2 / 9);
  expect(captureFB2Position(root, 0, { chapters: [] })).toBeNull();
});

it("restores the mapped word rather than a percentage of the chapter height", () => {
  const root = fixture();
  const create = document.createRange.bind(document);
  const ranges = [];
  vi.spyOn(document, "createRange").mockImplementation(() => {
    const range = create();
    range.getBoundingClientRect = () => ({ top: 500, height: 18 });
    ranges.push(range);
    return range;
  });
  const scroll = vi.spyOn(window, "scrollTo").mockImplementation(() => {});
  const path = root.querySelector("span").dataset.fb2Node;
  expect(restoreFB2Position(root, { path, offset: 2 })).toBe(true);
  expect(ranges[0].startOffset).toBe(3);
  expect(scroll).toHaveBeenCalledWith({ top: Math.max(0, window.scrollY + 500 - window.innerHeight * 0.35), left: 0, behavior: "instant" });
  expect(restoreFB2Position(root, { path, offset: 999 })).toBe(false);
});

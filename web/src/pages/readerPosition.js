const fraction = (value) => Math.max(0, Math.min(1, Number(value) || 0));

// KOReader's percentage does not contain a web chapter/scroll coordinate.
// Older records lack the marker; inconsistent coordinates identify those too.
const overallOnly = (saved, webOverall) => saved?.locator?.startsWith("kosync:")
  || saved?.locator?.startsWith("fb2:")
  || (!saved?.locator && Number(saved?.progress) > 0 && Math.abs(fraction(saved.progress) - webOverall) > 0.000001);

export const restoreChapterPosition = (saved, count) => {
  if (count < 1) return { index: 0, ratio: 0 };
  const index = Math.max(0, Math.min(count - 1, Math.trunc(Number(saved?.chapter) || 0)));
  const ratio = fraction(saved?.position);
  if (saved?.anchor) return { index, ratio };
  if (!overallOnly(saved, (index + ratio) / count)) return { index, ratio };
  const scaled = fraction(saved?.progress) * count;
  const approximateIndex = Math.min(count - 1, Math.floor(scaled));
  return { index: approximateIndex, ratio: Math.min(1, scaled - approximateIndex) };
};

export const restorePdfPage = (saved, count) => {
  const page = Math.max(1, Math.min(count, Math.trunc(Number(saved?.chapter) || 1)));
  return overallOnly(saved, page / count)
    ? Math.max(1, Math.min(count, Math.ceil(fraction(saved?.progress) * count)))
    : page;
};

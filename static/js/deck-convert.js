// Converts a legacy reveal deck page (rendered by go-slide) into a .pptx for DeckCraft.
// Run inside the deck page; the caller (controllers/deck_convert.go, or a test) drives it:
//   deckConvert.prepare()            -> {count, width, height}
//   deckConvert.show(i)              -> slide i on screen, its rectangle in viewport pixels
//   deckConvert.collect()            -> text boxes and notes of the slide on screen
//   deckConvert.hideText(true|false) -> text glyphs hidden (background capture) or shown again
//   deckConvert.keep(slide)          -> stores one captured slide {bg: base64 PNG, boxes, notes}
//   deckConvert.build(title[, slides]) -> base64 .pptx of the given slides, or of the kept ones
// Each slide becomes a full-slide picture of everything except the text (shapes, cards, colours,
// bullets) with the text rebuilt on top as editable text boxes at the browser-computed positions.
(function () {
  const PX_PER_IN = 96;
  const PT_PER_PX = 0.75;
  let size = { w: 1280, h: 800 };

  function section() {
    return document.querySelector('.reveal .slides section.present:not(.stack)') ||
      document.querySelector('.reveal .slides section.present');
  }

  function prepare() {
    const R = window.Reveal;
    const cfg = R.getConfig();
    size = { w: cfg.width || 1280, h: cfg.height || 800 };
    // One slide at 1:1 scale, no animation, no chrome over it, every fragment visible.
    R.configure({ transition: 'none', backgroundTransition: 'none', controls: false, progress: false,
      slideNumber: false, margin: 0, minScale: 1, maxScale: 1, fragments: false });
    const st = document.createElement('style');
    st.textContent = '.reveal .fragment{opacity:1!important;visibility:visible!important;transform:none!important}' +
      'body>*:not(.reveal):not(script):not(style){display:none!important}' +
      '.reveal .slides section,.reveal .slides section *{transition:none!important;animation:none!important}' +
      '.reveal .controls,.reveal .progress,.reveal .slide-number,.reveal aside.notes{display:none!important}' +
      '.dc-hide-text{color:transparent!important;-webkit-text-fill-color:transparent!important;text-shadow:none!important;text-decoration-color:transparent!important}';
    document.head.appendChild(st);
    R.layout();
    return { count: R.getTotalSlides(), width: size.w, height: size.h };
  }

  function show(i) {
    const R = window.Reveal;
    const slides = R.getSlides();
    const idx = R.getIndices(slides[i]);
    R.slide(idx.h, idx.v || 0);
    R.layout();
    const r = section().getBoundingClientRect();
    return { x: r.left, y: r.top, width: r.width, height: r.height };
  }

  const BLOCK = /^(block|list-item|flex|grid|table|table-cell|table-row|table-row-group|table-header-group|table-footer-group|table-caption|flow-root)$/;

  // A text block is an element that holds text itself and has no block-level child holding text.
  function isTextBlock(el) {
    if (!BLOCK.test(getComputedStyle(el).display) && el !== section()) return false;
    let own = false;
    for (const n of el.childNodes) {
      if (n.nodeType === 3 && n.textContent.trim()) own = true;
      if (n.nodeType === 1 && n.textContent.trim()) {
        if (BLOCK.test(getComputedStyle(n).display)) return false;
        own = true;
      }
    }
    return own;
  }

  function hex(color) {
    const m = color.match(/rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?/);
    if (!m) return { color: '000000', transparency: 0 };
    const c = [m[1], m[2], m[3]].map(v => Math.round(+v).toString(16).padStart(2, '0')).join('');
    return { color: c.toUpperCase(), transparency: m[4] === undefined ? 0 : Math.round((1 - +m[4]) * 100) };
  }

  // Font and bold for a CSS family and weight, in the fonts the site loads into DeckCraft
  // (static/js/deckcraft-fonts.js): Inter by weight (it resolves "Inter Medium" and "Inter SemiBold",
  // and Inter bold is Inter Bold), JetBrains Mono for any monospace stack.
  const MONO = /mono|courier|consolas|menlo|sfmono|ui-monospace/i;
  function face(family, weight) {
    const first = family.split(',')[0].replace(/["']/g, '').trim();
    if (MONO.test(family)) return { fontFace: 'JetBrains Mono', bold: weight >= 600 };
    if (/^(Inter|system-ui|-apple-system|BlinkMacSystemFont|sans-serif|ui-sans-serif|Segoe UI)$/i.test(first)) {
      if (weight >= 700) return { fontFace: 'Inter', bold: true };
      if (weight >= 600) return { fontFace: 'Inter SemiBold', bold: false };
      if (weight >= 500) return { fontFace: 'Inter Medium', bold: false };
      return { fontFace: 'Inter', bold: false };
    }
    return { fontFace: first, bold: weight >= 600 };
  }

  function runOptions(el) {
    const cs = getComputedStyle(el);
    const col = hex(cs.color);
    const o = {
      ...face(cs.fontFamily, +cs.fontWeight), fontSize: +(parseFloat(cs.fontSize) * PT_PER_PX).toFixed(1),
      italic: cs.fontStyle === 'italic', color: col.color,
    };
    if (cs.textDecorationLine.includes('underline')) o.underline = { style: 'sng' };
    if (cs.letterSpacing !== 'normal' && parseFloat(cs.letterSpacing)) o.charSpacing = +(parseFloat(cs.letterSpacing) * PT_PER_PX).toFixed(1);
    if (col.transparency) o.transparency = col.transparency;
    const link = el.closest('a');
    if (link && link.href) o.hyperlink = { url: link.href };
    return o;
  }

  const canvas = document.createElement('canvas').getContext('2d');
  // The width of a space in the element's own font, so a gap can be turned into a number of spaces.
  function spaceWidth(cs) {
    canvas.font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
    const ls = parseFloat(cs.letterSpacing) || 0;
    return (canvas.measureText(' ').width || parseFloat(cs.fontSize) * 0.28) + ls;
  }

  // The block's words with the line each one landed on (its top) and where it starts and ends,
  // so lines and wide gaps come out as the browser laid them out.
  function words(block) {
    const out = [];
    const range = document.createRange();
    const walker = document.createTreeWalker(block, NodeFilter.SHOW_TEXT);
    for (let t = walker.nextNode(); t; t = walker.nextNode()) {
      const p = t.parentElement;
      if (!p || p.closest('aside.notes')) continue;
      const cs = getComputedStyle(p);
      if (cs.display === 'none' || cs.visibility === 'hidden') continue;
      const opts = runOptions(p);
      const key = JSON.stringify(opts);
      const raw = t.textContent;
      // Words, split after hyphens too: the browser may break a line inside "multi-kind".
      for (const m of raw.matchAll(/[^\s-]+-*|-+/g)) {
        range.setStart(t, m.index);
        range.setEnd(t, m.index + m[0].length);
        const rects = range.getClientRects();
        if (!rects.length) continue;
        let text = m[0];
        if (cs.textTransform === 'uppercase') text = text.toUpperCase();
        if (cs.textTransform === 'lowercase') text = text.toLowerCase();
        out.push({ text, opts, key, top: rects[0].top, bottom: rects[0].bottom, left: rects[0].left, right: rects[rects.length - 1].right,
          size: parseFloat(cs.fontSize), space: spaceWidth(cs) });
      }
    }
    return out;
  }

  // Runs for PptxGenJS: words of one style on one line merged, a break after each line but the last,
  // and extra spaces where the browser left a gap wider than a space (spaced-out names, tabs).
  function runs(block) {
    const ws = words(block);
    const out = [];
    for (let i = 0; i < ws.length; i++) {
      const w = ws[i], prev = ws[i - 1];
      // Same line when the two words overlap vertically (a small run beside a large one shares the line).
      const newLine = prev && (w.top + w.bottom) / 2 > prev.bottom && (prev.top + prev.bottom) / 2 < w.top;
      let sep = '';
      if (prev && !newLine) {
        const gap = w.left - prev.right;
        sep = gap > w.space * 0.3 ? ' '.repeat(Math.max(1, Math.min(12, Math.round(gap / w.space)))) : '';
      }
      if (newLine && out.length) out[out.length - 1].options.breakLine = true;
      const last = out[out.length - 1];
      if (last && !newLine && last.key === w.key && !last.options.breakLine) last.text += sep + w.text;
      else out.push({ text: (newLine ? '' : sep) + w.text, key: w.key, options: { ...w.opts } });
    }
    const res = out.map(r => ({ text: r.text, options: r.options }));
    res.extent = ws.length ? { left: Math.min(...ws.map(w => w.left)), right: Math.max(...ws.map(w => w.right)) } : null;
    return res;
  }

  function collect() {
    const sec = section();
    const base = sec.getBoundingClientRect();
    const boxes = [];
    const seen = new Set();
    const all = [sec, ...sec.querySelectorAll('*')];
    for (const el of all) {
      if (el.closest('aside.notes') || seen.has(el) || !el.textContent.trim() || !isTextBlock(el)) continue;
      if ([...seen].some(s => s.contains(el))) continue;
      seen.add(el);
      const cs = getComputedStyle(el);
      if (cs.visibility === 'hidden' || +cs.opacity === 0) continue;
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) continue;
      const rs = runs(el);
      if (!rs.length) continue;
      const pl = parseFloat(cs.paddingLeft), pr = parseFloat(cs.paddingRight), pt = parseFloat(cs.paddingTop), pb = parseFloat(cs.paddingBottom);
      const lh = cs.lineHeight === 'normal' ? parseFloat(cs.fontSize) * 1.2 : parseFloat(cs.lineHeight);
      const align = { center: 'center', right: 'right', end: 'right', justify: 'justify' }[cs.textAlign] || 'left';
      const li = el.tagName === 'LI';
      let x = r.left - base.left + pl, w = (r.width - pl - pr) * 1.04 + 2, al = align;
      // One line: a wider box anchored where the browser drew the text (its start, centre or end,
      // as the line is aligned), so a font a little wider than the browser's can't wrap or move it.
      if (!rs.some(run => run.options.breakLine) && rs.extent) {
        const tw = rs.extent.right - rs.extent.left, left = rs.extent.left - base.left;
        const mid = Math.abs((rs.extent.left - r.left) - (r.right - rs.extent.right)) < 3 && rs.extent.left - r.left > 3;
        al = mid ? 'center' : align;
        w = tw * 1.3 + 16;
        x = al === 'center' ? left - (w - tw) / 2 : al === 'right' ? left + tw - w : left;
      }
      boxes.push({
        x, y: r.top - base.top + pt, w, h: r.height - pt - pb,
        runs: rs, align: al, lineSpacing: +(lh * PT_PER_PX).toFixed(1), li,
        valign: cs.display === 'table-cell' && cs.verticalAlign === 'middle' ? 'middle' : 'top',
      });
    }
    const notes = [...sec.querySelectorAll('aside.notes')].map(n => n.innerText.trim()).filter(Boolean).join('\n\n');
    return { boxes, notes };
  }

  // Hides only the text glyphs (wrapping each text node), so ::before bullets, borders and
  // backgrounds stay in the captured picture.
  function hideText(on) {
    const sec = section();
    if (!on) {
      sec.querySelectorAll('span.dc-hide-text').forEach(s => s.replaceWith(...s.childNodes));
      sec.normalize();
      return;
    }
    const walker = document.createTreeWalker(sec, NodeFilter.SHOW_TEXT);
    const nodes = [];
    for (let t = walker.nextNode(); t; t = walker.nextNode()) if (t.textContent.trim()) nodes.push(t);
    for (const t of nodes) {
      const s = document.createElement('span');
      s.className = 'dc-hide-text';
      t.replaceWith(s);
      s.appendChild(t);
    }
  }

  function inch(px) { return +(px / PX_PER_IN).toFixed(3); }

  const kept = [];
  function keep(slide) { kept.push(slide); return kept.length; }

  async function build(title, slides) {
    slides = slides || kept;
    const pptx = new window.PptxGenJS();
    pptx.defineLayout({ name: 'DECK', width: inch(size.w), height: inch(size.h) });
    pptx.layout = 'DECK';
    pptx.title = title;
    for (const s of slides) {
      const slide = pptx.addSlide();
      const data = s.bg.startsWith('data:') ? s.bg : 'data:image/png;base64,' + s.bg;
      slide.addImage({ data, x: 0, y: 0, w: inch(size.w), h: inch(size.h) });
      for (const b of s.boxes) {
        const rs = b.runs.map(r => ({ text: r.text, options: { ...r.options, bullet: false } }));
        slide.addText(rs, {
          x: inch(b.x), y: inch(b.y), w: inch(b.w), h: inch(b.h),
          margin: 0, valign: b.valign || 'top', align: b.align, lineSpacing: b.lineSpacing, fit: 'none', wrap: true,
        });
      }
      if (s.notes) slide.addNotes(s.notes);
    }
    return pptx.write({ outputType: 'base64' });
  }

  window.deckConvert = { prepare, show, collect, hideText, keep, build };
})();

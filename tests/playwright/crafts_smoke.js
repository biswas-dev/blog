// Smoke test for the vendored craft web builds (crafts.lock): run it after scripts/crafts.sh bump.
//   scripts/crafts.sh fetch && node tests/playwright/crafts_smoke.js
// It serves static/ on a local port with a bare same-origin host page and checks, per app, the
// ?host=parent contract the blog relies on (templates/slide-editor.gohtml, slide-deck.gohtml,
// paper-editor.gohtml): ready -> open bytes -> save bytes back. CHROME=<path> picks a browser.
const http = require('http');
const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const STATIC = path.join(__dirname, '..', '..', 'static');
const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.wasm': 'application/wasm', '.onnx': 'application/octet-stream' };

const HOST = `<!doctype html><body style="margin:0"><iframe id="f" style="width:1200px;height:760px;border:0"></iframe><script>
const app = new URLSearchParams(location.search).get('app');
window.log = []; window.saves = [];
window.openFile = (name, bytes) => f.contentWindow.postMessage({ type: app + ':open', name, bytes }, location.origin);
addEventListener('message', (e) => {
  if (e.source !== f.contentWindow || !e.data || typeof e.data.type !== 'string') return;
  window.log.push(e.data.type);
  if (e.data.type === app + ':save') window.saves.push({ name: e.data.name, saveAs: e.data.saveAs, bytes: Array.from(new Uint8Array(e.data.bytes)) });
});
f.src = '/static/vendor/' + app + '/index.html?host=parent' + (app === 'pdfcraft' ? '&control' : '');
</script>`;

// A one-page PDF with a correct xref table, carrying a highlight with a note and a reply: the
// standard annotations PdfCraft writes for paper markup (/Contents, /T, /IRT).
function tinyPDF() {
  const objs = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Annots [4 0 R 5 0 R] >>',
    '<< /Type /Annot /Subtype /Highlight /Rect [20 150 120 170] /QuadPoints [20 170 120 170 20 150 120 150] /C [1 0.94 0] /T (Smoke) /NM (smoke-hl) /Contents (Smoke note) /P 3 0 R >>',
    '<< /Type /Annot /Subtype /Text /Rect [20 150 120 170] /IRT 4 0 R /T (Smoke) /NM (smoke-reply) /Contents (Smoke reply) /P 3 0 R >>'];
  let out = '%PDF-1.4\n';
  const offsets = objs.map((o, i) => { const at = out.length; out += `${i + 1} 0 obj\n${o}\nendobj\n`; return at; });
  const xref = out.length;
  out += `xref\n0 ${objs.length + 1}\n0000000000 65535 f \n` + offsets.map((o) => `${String(o).padStart(10, '0')} 00000 n \n`).join('');
  out += `trailer\n<< /Size ${objs.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Array.from(Buffer.from(out, 'latin1'));
}

function serve() {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://x');
    if (url.pathname === '/host.html') { res.writeHead(200, { 'Content-Type': 'text/html' }); return res.end(HOST); }
    const file = path.join(STATIC, path.normalize(url.pathname.replace(/^\/static\//, '')));
    if (!file.startsWith(STATIC) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); return res.end(); }
    res.writeHead(200, { 'Content-Type': TYPES[path.extname(file)] || 'application/octet-stream' });
    fs.createReadStream(file).pipe(res);
  });
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(server)));
}

const assert = (ok, msg) => { if (!ok) throw new Error(msg); console.log('  ok -', msg); };

(async () => {
  for (const app of ['deckcraft', 'pdfcraft']) {
    if (!fs.existsSync(path.join(STATIC, 'vendor', app, 'index.html'))) throw new Error(`static/vendor/${app} missing: run scripts/crafts.sh fetch`);
  }
  const server = await serve();
  const base = `http://127.0.0.1:${server.address().port}/host.html`;
  const browser = await chromium.launch({ executablePath: process.env.CHROME || undefined, args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] });
  const page = await browser.newPage({ viewport: { width: 1200, height: 760 } });
  const api = (app, fn, ...args) => page.evaluate(([app, fn, args]) => document.getElementById('f').contentWindow[app][fn](...args), [app, fn, args]);
  const ready = (app) => page.waitForFunction((a) => window.log.includes(a + ':ready') || window.log.includes(a + ':failed'), app, { timeout: 120000 })
    .then(() => page.evaluate(() => window.log)).then((log) => assert(log.includes(app + ':ready'), `${app} posts :ready`));
  const lastSave = (n) => page.waitForFunction((n) => window.saves.length >= n, n, { timeout: 30000 }).then(() => page.evaluate(() => window.saves.at(-1)));
  let failed = false;
  try {
    console.log('deckcraft');
    await page.goto(base + '?app=deckcraft'); await ready('deckcraft');
    await api('deckcraft', 'execute', 'file.save', {});
    const deck = await lastSave(1);
    assert(deck.saveAs === true && /\.deckcraft$/.test(deck.name), `new deck saves as ${deck.name} (saveAs)`);
    assert(Buffer.from(deck.bytes.slice(0, 4)).toString('latin1') === 'PK\x03\x04', 'deck file is a zip package');
    await page.reload(); await ready('deckcraft');
    await page.evaluate((d) => window.openFile(d.name, new Uint8Array(d.bytes).buffer), deck);
    await page.waitForFunction((name) => document.getElementById('f').contentWindow.deckcraft.uiInspect()
      .then((ui) => (window.ui = ui)) && window.ui && window.ui.documents.some((d) => (d.path || '').includes(name)), deck.name, { timeout: 30000, polling: 250 });
    assert(true, 'reopens the saved deck by name');
    await api('deckcraft', 'execute', 'show.start', { from: 0 });
    const ui = await api('deckcraft', 'uiInspect');
    assert(!!ui.showing, 'show.start runs the slide show (uiInspect().showing)');
    await api('deckcraft', 'execute', 'file.save', {});
    const again = await lastSave(1);
    assert(again.name === deck.name && again.saveAs === false, 'Save of an opened deck keeps its name (saveAs false)');

    console.log('pdfcraft');
    await page.goto(base + '?app=pdfcraft'); await ready('pdfcraft');
    await page.evaluate((b) => window.openFile('paper.pdf', new Uint8Array(b).buffer), tinyPDF());
    await page.waitForTimeout(2000);
    await api('pdfcraft', 'execute', 'comment.list', {});
    await page.waitForTimeout(1000);
    const texts = (await api('pdfcraft', 'uiInspect')).widgets.map((w) => w.value || w.label || '');
    assert(['Smoke note', 'Smoke reply'].every((t) => texts.some((x) => x.includes(t))), 'comment list shows the note and its reply from the PDF');
    await api('pdfcraft', 'execute', 'file.save', {});
    const pdf = await lastSave(1);
    assert(pdf.name === 'paper.pdf', 'save keeps the name the host sent');
    assert(Buffer.from(pdf.bytes.slice(0, 5)).toString('latin1') === '%PDF-', 'saved bytes are a PDF');
    assert(Buffer.from(pdf.bytes).toString('latin1').includes('/IRT'), 'saved PDF keeps the reply');
  } catch (err) {
    failed = true;
    console.error('FAIL:', err.message);
  } finally {
    await browser.close();
    server.close();
  }
  process.exit(failed ? 1 : 0);
})();

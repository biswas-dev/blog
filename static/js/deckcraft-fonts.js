// Loads the fonts decks are drawn in into a DeckCraft frame (its font.add command). The web build
// bundles a single face, so without these Inter and monospace text fall back to it.
// Call after deckcraft:ready and before opening a file: await deckcraftFonts(frame.contentWindow.deckcraft)
(function () {
  const FILES = ['Inter-Regular', 'Inter-Medium', 'Inter-SemiBold', 'Inter-Bold', 'Inter-Italic',
    'JetBrainsMono-Regular', 'JetBrainsMono-Bold'];
  const VERSION = '1';

  function base64(buf) {
    return new Promise(function (ok, fail) {
      const r = new FileReader();
      r.onload = function () { ok(String(r.result).split(',')[1]); };
      r.onerror = fail;
      r.readAsDataURL(new Blob([buf]));
    });
  }

  window.deckcraftFonts = async function (dc) {
    const files = await Promise.all(FILES.map(function (n) {
      return fetch('/static/fonts/deckcraft/' + n + '.ttf?v=' + VERSION)
        .then(function (r) { return r.ok ? r.arrayBuffer() : null; }).catch(function () { return null; });
    }));
    for (const buf of files) {
      // A font that fails to load leaves its text in DeckCraft's own face; the deck still opens.
      if (buf) await dc.execute('font.add', { data: await base64(buf) }).catch(function () {});
    }
  };
})();

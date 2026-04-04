// Unified keyboard navigation for content pages
// Shortcuts:
//   Cmd+K / Ctrl+K   → global search (posts + slides + guides, navigate anywhere)
//   /                → local find-in-page (search within current page content)
//   j                → previous section heading (or prev slide in Reveal.js)
//   k                → next section heading (or next slide in Reveal.js)
//   n / Shift+N      → next/prev match (when local find is active)
//   Esc              → close search / clear find
(function() {
  'use strict';

  // ---------- Section heading cache ----------
  var headingsCache = null;
  function getHeadings() {
    if (headingsCache) return headingsCache;
    var selectors = [
      '#article-content h1', '#article-content h2', '#article-content h3',
      '.prose h1', '.prose h2', '.prose h3',
      'article h1', 'article h2', 'article h3',
      '.guide-content h2', '.guide-content h3'
    ];
    var nodes = document.querySelectorAll(selectors.join(','));
    var seen = new Set();
    var result = [];
    nodes.forEach(function(n) {
      if (!seen.has(n)) { seen.add(n); result.push(n); }
    });
    headingsCache = result;
    return result;
  }

  function currentHeadingIdx() {
    var hs = getHeadings();
    if (hs.length === 0) return -1;
    var scrollY = window.scrollY + 120;
    var idx = -1;
    for (var i = 0; i < hs.length; i++) {
      var top = hs[i].getBoundingClientRect().top + window.scrollY;
      if (top <= scrollY) idx = i; else break;
    }
    return idx;
  }

  function scrollToHeading(dir) {
    var hs = getHeadings();
    if (hs.length === 0) return;
    var idx = currentHeadingIdx();
    var target = dir > 0 ? Math.min(idx + 1, hs.length - 1) : Math.max(idx - 1, 0);
    if (target === idx) return;
    hs[target].scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function escapeHtml(s) {
    return String(s || '').replace(/[&<>"']/g, function(c) {
      return { '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c];
    });
  }

  // ============================================
  //   GLOBAL SEARCH (Cmd+K / Ctrl+K)
  // ============================================
  var gOverlay, gInput, gResults, gSelectedIdx = -1, gCurrentResults = [], gTimer;

  function ensureGlobalOverlay() {
    if (gOverlay) return;
    gOverlay = document.createElement('div');
    gOverlay.id = 'keynav-search';
    gOverlay.innerHTML =
      '<div class="keynav-backdrop"></div>' +
      '<div class="keynav-modal">' +
        '<div class="keynav-input-wrap">' +
          '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><path d="m21 21-4.3-4.3"></path></svg>' +
          '<input type="search" placeholder="Search all posts, slides, and guides..." autocomplete="off" spellcheck="false">' +
          '<kbd>Esc</kbd>' +
        '</div>' +
        '<div class="keynav-results"></div>' +
        '<div class="keynav-footer">' +
          '<span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>' +
          '<span><kbd>Enter</kbd> open</span>' +
          '<span><kbd>⌘K</kbd> global  <kbd>/</kbd> find in page</span>' +
        '</div>' +
      '</div>';
    document.body.appendChild(gOverlay);
    gOverlay.querySelector('.keynav-backdrop').addEventListener('click', closeGlobal);
    gInput = gOverlay.querySelector('input');
    gResults = gOverlay.querySelector('.keynav-results');
    gInput.addEventListener('input', onGlobalInput);
    gInput.addEventListener('keydown', onGlobalInputKey);
  }

  function openGlobal() {
    ensureGlobalOverlay();
    gOverlay.classList.add('active');
    gSelectedIdx = -1;
    gCurrentResults = [];
    gResults.innerHTML = '<div class="keynav-hint">Start typing to search posts, slides, and guides...</div>';
    setTimeout(function() { gInput.focus(); gInput.select(); }, 30);
  }

  function closeGlobal() { if (gOverlay) gOverlay.classList.remove('active'); }

  function onGlobalInput() {
    clearTimeout(gTimer);
    var q = gInput.value.trim();
    if (!q) {
      gCurrentResults = [];
      gResults.innerHTML = '<div class="keynav-hint">Start typing to search posts, slides, and guides...</div>';
      return;
    }
    gTimer = setTimeout(function() { doGlobalSearch(q); }, 150);
  }

  function doGlobalSearch(q) {
    fetch('/api/search?q=' + encodeURIComponent(q))
      .then(function(r) { return r.ok ? r.json() : null; })
      .then(function(data) {
        if (!data) return;
        var posts = (data.posts || []).map(function(p) {
          return { type: 'post', title: p.title, slug: p.slug, excerpt: p.excerpt, url: '/blog/' + p.slug };
        });
        var slides = (data.slides || []).map(function(s) {
          return { type: 'slide', title: s.title, slug: s.slug, excerpt: s.excerpt, url: '/slides/' + s.slug };
        });
        var guides = (data.guides || []).map(function(g) {
          return { type: 'guide', title: g.title, slug: g.slug, excerpt: g.excerpt, url: '/guides/' + g.slug };
        });
        gCurrentResults = posts.concat(guides).concat(slides);
        renderGlobalResults();
      });
  }

  function renderGlobalResults() {
    if (gCurrentResults.length === 0) {
      gResults.innerHTML = '<div class="keynav-hint">No results</div>';
      gSelectedIdx = -1;
      return;
    }
    gSelectedIdx = 0;
    gResults.innerHTML = gCurrentResults.map(function(r, i) {
      var excerpt = (r.excerpt || '').substring(0, 120);
      return '<a href="' + escapeHtml(r.url) + '" class="keynav-result" data-idx="' + i + '">' +
        '<span class="keynav-type keynav-type-' + r.type + '">' + r.type + '</span>' +
        '<div class="keynav-meta">' +
          '<div class="keynav-title">' + escapeHtml(r.title) + '</div>' +
          '<div class="keynav-excerpt">' + escapeHtml(excerpt) + '</div>' +
        '</div>' +
      '</a>';
    }).join('');
    updateGlobalSelection();
  }

  function updateGlobalSelection() {
    var items = gResults.querySelectorAll('.keynav-result');
    items.forEach(function(el) {
      var isSel = parseInt(el.dataset.idx, 10) === gSelectedIdx;
      el.classList.toggle('selected', isSel);
      if (isSel) el.scrollIntoView({ block: 'nearest' });
    });
  }

  function onGlobalInputKey(e) {
    if (e.key === 'Escape') { closeGlobal(); return; }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (gCurrentResults.length === 0) return;
      gSelectedIdx = (gSelectedIdx + 1) % gCurrentResults.length;
      updateGlobalSelection();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (gCurrentResults.length === 0) return;
      gSelectedIdx = (gSelectedIdx - 1 + gCurrentResults.length) % gCurrentResults.length;
      updateGlobalSelection();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (gCurrentResults[gSelectedIdx]) window.location.href = gCurrentResults[gSelectedIdx].url;
    }
  }

  // ============================================
  //   LOCAL FIND-IN-PAGE ( / )
  // ============================================
  var lBar, lInput, lCounter, lMatches = [], lActiveIdx = -1, lOriginal = null;

  function getContentRoot() {
    return document.querySelector('#article-content') ||
           document.querySelector('article') ||
           document.querySelector('.prose') ||
           document.querySelector('main') ||
           document.body;
  }

  function ensureLocalBar() {
    if (lBar) return;
    lBar = document.createElement('div');
    lBar.id = 'keynav-find';
    lBar.innerHTML =
      '<div class="keynav-find-bar">' +
        '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><path d="m21 21-4.3-4.3"></path></svg>' +
        '<input type="search" placeholder="Find in page..." autocomplete="off" spellcheck="false">' +
        '<span class="keynav-find-counter">0 / 0</span>' +
        '<button class="keynav-find-prev" title="Previous (Shift+Enter)">↑</button>' +
        '<button class="keynav-find-next" title="Next (Enter)">↓</button>' +
        '<button class="keynav-find-close" title="Close (Esc)">✕</button>' +
      '</div>';
    document.body.appendChild(lBar);
    lInput = lBar.querySelector('input');
    lCounter = lBar.querySelector('.keynav-find-counter');
    lInput.addEventListener('input', onLocalInput);
    lInput.addEventListener('keydown', onLocalKey);
    lBar.querySelector('.keynav-find-prev').addEventListener('click', function() { cycleMatch(-1); });
    lBar.querySelector('.keynav-find-next').addEventListener('click', function() { cycleMatch(1); });
    lBar.querySelector('.keynav-find-close').addEventListener('click', closeLocal);
  }

  function openLocal() {
    ensureLocalBar();
    lBar.classList.add('active');
    setTimeout(function() { lInput.focus(); lInput.select(); }, 30);
  }

  function closeLocal() {
    if (!lBar) return;
    lBar.classList.remove('active');
    clearHighlights();
    lInput.value = '';
    lMatches = [];
    lActiveIdx = -1;
    lCounter.textContent = '0 / 0';
  }

  function clearHighlights() {
    var marks = document.querySelectorAll('mark.keynav-match');
    marks.forEach(function(m) {
      var p = m.parentNode;
      while (m.firstChild) p.insertBefore(m.firstChild, m);
      p.removeChild(m);
      p.normalize();
    });
  }

  function highlightInNode(node, regex) {
    if (node.nodeType === 3) {
      // text node
      var text = node.nodeValue;
      var m = regex.exec(text);
      if (!m) return;
      var frag = document.createDocumentFragment();
      var lastEnd = 0;
      regex.lastIndex = 0;
      while ((m = regex.exec(text)) !== null) {
        if (m.index > lastEnd) {
          frag.appendChild(document.createTextNode(text.slice(lastEnd, m.index)));
        }
        var mark = document.createElement('mark');
        mark.className = 'keynav-match';
        mark.textContent = m[0];
        frag.appendChild(mark);
        lMatches.push(mark);
        lastEnd = m.index + m[0].length;
        if (m.index === regex.lastIndex) regex.lastIndex++;
      }
      if (lastEnd < text.length) frag.appendChild(document.createTextNode(text.slice(lastEnd)));
      node.parentNode.replaceChild(frag, node);
    } else if (node.nodeType === 1) {
      var tag = node.tagName;
      if (tag === 'SCRIPT' || tag === 'STYLE' || tag === 'MARK' || tag === 'CODE' && node.closest('pre')) return;
      if (node.id === 'keynav-find' || node.id === 'keynav-search') return;
      var children = Array.from(node.childNodes);
      children.forEach(function(c) { highlightInNode(c, regex); });
    }
  }

  function onLocalInput() {
    clearHighlights();
    lMatches = [];
    lActiveIdx = -1;
    var q = lInput.value;
    if (!q || q.length < 2) { lCounter.textContent = '0 / 0'; return; }
    var escaped = q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    var regex = new RegExp(escaped, 'gi');
    highlightInNode(getContentRoot(), regex);
    lCounter.textContent = lMatches.length > 0 ? '1 / ' + lMatches.length : '0 / 0';
    if (lMatches.length > 0) {
      lActiveIdx = 0;
      activateMatch();
    }
  }

  function activateMatch() {
    lMatches.forEach(function(m, i) {
      m.classList.toggle('active', i === lActiveIdx);
    });
    if (lMatches[lActiveIdx]) {
      lMatches[lActiveIdx].scrollIntoView({ behavior: 'smooth', block: 'center' });
      lCounter.textContent = (lActiveIdx + 1) + ' / ' + lMatches.length;
    }
  }

  function cycleMatch(dir) {
    if (lMatches.length === 0) return;
    lActiveIdx = (lActiveIdx + dir + lMatches.length) % lMatches.length;
    activateMatch();
  }

  function onLocalKey(e) {
    if (e.key === 'Escape') { closeLocal(); return; }
    if (e.key === 'Enter') {
      e.preventDefault();
      cycleMatch(e.shiftKey ? -1 : 1);
    }
  }

  // ============================================
  //   GLOBAL KEY HANDLERS
  // ============================================
  document.addEventListener('keydown', function(e) {
    // Cmd+K or Ctrl+K → global search (always available)
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      openGlobal();
      return;
    }

    // Ignore when typing
    var tag = e.target && e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    if (e.target && e.target.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === '/') {
      e.preventDefault();
      openLocal();
    } else if (e.key === 'Escape') {
      closeGlobal();
      closeLocal();
    } else if (e.key === 'j' || e.key === 'J') {
      if (window.Reveal && typeof window.Reveal.prev === 'function') {
        e.preventDefault();
        window.Reveal.prev();
      } else if (getHeadings().length > 0) {
        e.preventDefault();
        scrollToHeading(-1);
      }
    } else if (e.key === 'k' || e.key === 'K') {
      if (window.Reveal && typeof window.Reveal.next === 'function') {
        e.preventDefault();
        window.Reveal.next();
      } else if (getHeadings().length > 0) {
        e.preventDefault();
        scrollToHeading(1);
      }
    } else if (e.key === 'n' && lBar && lBar.classList.contains('active')) {
      e.preventDefault();
      cycleMatch(e.shiftKey ? -1 : 1);
    }
  });
})();

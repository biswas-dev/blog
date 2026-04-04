// Unified keyboard navigation for content pages
// Shortcuts:
//   /      → open search overlay (searches posts + slides)
//   j      → previous section heading (or prev slide in Reveal.js)
//   k      → next section heading (or next slide in Reveal.js)
//   Esc    → close search
//
// Search results: arrow keys to navigate, Enter to go to selected result
(function() {
  'use strict';

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
      var rect = hs[i].getBoundingClientRect();
      var top = rect.top + window.scrollY;
      if (top <= scrollY) idx = i;
      else break;
    }
    return idx;
  }

  function scrollToHeading(dir) {
    var hs = getHeadings();
    if (hs.length === 0) return;
    var idx = currentHeadingIdx();
    var target;
    if (dir > 0) target = Math.min(idx + 1, hs.length - 1);
    else target = Math.max(idx - 1, 0);
    if (target === idx) return;
    hs[target].scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  // ---- Search Overlay ----
  var overlay, input, resultsEl, selectedIdx = -1, currentResults = [], searchTimer;

  function ensureOverlay() {
    if (overlay) return;
    overlay = document.createElement('div');
    overlay.id = 'keynav-search';
    overlay.innerHTML =
      '<div class="keynav-backdrop"></div>' +
      '<div class="keynav-modal">' +
        '<div class="keynav-input-wrap">' +
          '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><path d="m21 21-4.3-4.3"></path></svg>' +
          '<input type="search" placeholder="Search posts and slides..." autocomplete="off" spellcheck="false">' +
          '<kbd>Esc</kbd>' +
        '</div>' +
        '<div class="keynav-results"></div>' +
        '<div class="keynav-footer">' +
          '<span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>' +
          '<span><kbd>Enter</kbd> open</span>' +
          '<span><kbd>/</kbd> open search anywhere</span>' +
        '</div>' +
      '</div>';
    document.body.appendChild(overlay);
    overlay.querySelector('.keynav-backdrop').addEventListener('click', closeSearch);
    input = overlay.querySelector('input');
    resultsEl = overlay.querySelector('.keynav-results');
    input.addEventListener('input', onInput);
    input.addEventListener('keydown', onInputKey);
  }

  function openSearch() {
    ensureOverlay();
    overlay.classList.add('active');
    selectedIdx = -1;
    currentResults = [];
    resultsEl.innerHTML = '<div class="keynav-hint">Start typing to search...</div>';
    setTimeout(function() { input.focus(); input.select(); }, 30);
  }

  function closeSearch() {
    if (overlay) overlay.classList.remove('active');
  }

  function escapeHtml(s) {
    return String(s || '').replace(/[&<>"']/g, function(c) {
      return { '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c];
    });
  }

  function onInput() {
    clearTimeout(searchTimer);
    var q = input.value.trim();
    if (!q) {
      currentResults = [];
      resultsEl.innerHTML = '<div class="keynav-hint">Start typing to search...</div>';
      return;
    }
    searchTimer = setTimeout(function() { doSearch(q); }, 150);
  }

  function doSearch(q) {
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
        currentResults = posts.concat(slides);
        renderResults();
      });
  }

  function renderResults() {
    if (currentResults.length === 0) {
      resultsEl.innerHTML = '<div class="keynav-hint">No results</div>';
      selectedIdx = -1;
      return;
    }
    selectedIdx = 0;
    var html = currentResults.map(function(r, i) {
      var excerpt = (r.excerpt || '').substring(0, 120);
      return '<a href="' + escapeHtml(r.url) + '" class="keynav-result" data-idx="' + i + '">' +
        '<span class="keynav-type keynav-type-' + r.type + '">' + r.type + '</span>' +
        '<div class="keynav-meta">' +
          '<div class="keynav-title">' + escapeHtml(r.title) + '</div>' +
          '<div class="keynav-excerpt">' + escapeHtml(excerpt) + '</div>' +
        '</div>' +
      '</a>';
    }).join('');
    resultsEl.innerHTML = html;
    updateSelection();
  }

  function updateSelection() {
    var items = resultsEl.querySelectorAll('.keynav-result');
    items.forEach(function(el) {
      var isSel = parseInt(el.dataset.idx, 10) === selectedIdx;
      el.classList.toggle('selected', isSel);
      if (isSel) el.scrollIntoView({ block: 'nearest' });
    });
  }

  function onInputKey(e) {
    if (e.key === 'Escape') { closeSearch(); return; }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (currentResults.length === 0) return;
      selectedIdx = (selectedIdx + 1) % currentResults.length;
      updateSelection();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (currentResults.length === 0) return;
      selectedIdx = (selectedIdx - 1 + currentResults.length) % currentResults.length;
      updateSelection();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (currentResults[selectedIdx]) {
        window.location.href = currentResults[selectedIdx].url;
      }
    }
  }

  // ---- Global keydown ----
  document.addEventListener('keydown', function(e) {
    // Ignore when typing
    var tag = e.target && e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    if (e.target && e.target.isContentEditable) return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    if (e.key === '/') {
      e.preventDefault();
      openSearch();
    } else if (e.key === 'Escape') {
      closeSearch();
    } else if (e.key === 'j' || e.key === 'J') {
      // j = back (previous heading/slide)
      if (window.Reveal && typeof window.Reveal.prev === 'function') {
        e.preventDefault();
        window.Reveal.prev();
      } else if (getHeadings().length > 0) {
        e.preventDefault();
        scrollToHeading(-1);
      }
    } else if (e.key === 'k' || e.key === 'K') {
      // k = forward (next heading/slide)
      if (window.Reveal && typeof window.Reveal.next === 'function') {
        e.preventDefault();
        window.Reveal.next();
      } else if (getHeadings().length > 0) {
        e.preventDefault();
        scrollToHeading(1);
      }
    }
  });
})();

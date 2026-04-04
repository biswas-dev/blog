// Reader engagement tracker
// Captures: time on page (active + total), max scroll %, per-section attention
// Sends beacons via navigator.sendBeacon on heartbeat/hide/unload.
(function() {
  'use strict';

  // Only track on actual content pages
  var path = location.pathname;
  if (!/^\/(blog|guides|slides|docs|about|tags|users)\b/.test(path) && path !== '/') return;

  // Generate a session ID (uuid v4, simple)
  function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
      var r = Math.random() * 16 | 0;
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    });
  }

  var sessionId = uuid();
  var startedAt = Date.now();
  var activeSeconds = 0;
  var totalSeconds = 0;
  var maxScrollPct = 0;
  var isVisible = !document.hidden;
  var lastTickMs = Date.now();
  var lastFlushMs = 0;
  var flushed = false;

  // Section tracking: every h2/h3 inside #article-content or .prose
  var sections = [];
  var sectionMap = {};
  function initSections() {
    var containers = document.querySelectorAll('#article-content, .prose, article');
    var idx = 0;
    containers.forEach(function(c) {
      var headings = c.querySelectorAll('h1, h2, h3');
      headings.forEach(function(h) {
        if (!h.id) {
          h.id = 'sec-' + idx;
        }
        var text = (h.textContent || '').trim().slice(0, 120);
        if (!text) return;
        var entry = { id: h.id, text: text, order: idx, active_seconds: 0, viewed: false, _visible: false };
        sections.push(entry);
        sectionMap[h.id] = entry;
        idx++;
      });
    });
  }

  // Intersection Observer: track which heading is currently in view
  var currentSectionId = null;
  function initObserver() {
    if (!window.IntersectionObserver || sections.length === 0) return;
    var io = new IntersectionObserver(function(entries) {
      entries.forEach(function(e) {
        var entry = sectionMap[e.target.id];
        if (!entry) return;
        entry._visible = e.isIntersecting;
        if (e.isIntersecting) {
          entry.viewed = true;
        }
      });
    }, { rootMargin: '-20% 0px -60% 0px', threshold: 0 });
    sections.forEach(function(s) {
      var el = document.getElementById(s.id);
      if (el) io.observe(el);
    });
  }

  // Scroll tracking
  function updateScroll() {
    var doc = document.documentElement;
    var scrollTop = window.scrollY || doc.scrollTop || 0;
    var scrollHeight = doc.scrollHeight - window.innerHeight;
    if (scrollHeight <= 0) return;
    var pct = Math.round((scrollTop / scrollHeight) * 100);
    if (pct > maxScrollPct) maxScrollPct = Math.min(100, pct);
  }

  // Tick: accumulate active time every 500ms
  function tick() {
    var now = Date.now();
    var delta = (now - lastTickMs) / 1000;
    lastTickMs = now;
    if (delta > 2) delta = 0; // tab was suspended; don't count
    totalSeconds += delta;
    if (isVisible && hasRecentActivity()) {
      activeSeconds += delta;
      // Attribute active time to visible sections
      sections.forEach(function(s) {
        if (s._visible) s.active_seconds += delta;
      });
    }
    updateScroll();
    // Flush every 30s while active
    if (now - lastFlushMs > 30000 && activeSeconds > 2) {
      flush(false);
    }
  }

  // Activity detection (idle > 60s counts as inactive)
  var lastActivityMs = Date.now();
  function hasRecentActivity() {
    return (Date.now() - lastActivityMs) < 60000;
  }
  ['mousemove', 'keydown', 'scroll', 'click', 'touchstart'].forEach(function(ev) {
    window.addEventListener(ev, function() { lastActivityMs = Date.now(); }, { passive: true });
  });

  // Visibility
  document.addEventListener('visibilitychange', function() {
    isVisible = !document.hidden;
    if (document.hidden) flush(false);
  });

  // Flush
  function flush(final) {
    lastFlushMs = Date.now();
    var payload = {
      session_id: sessionId,
      path: path,
      referrer: document.referrer || '',
      started_at: startedAt,
      active_seconds: Math.round(activeSeconds),
      total_seconds: Math.round(totalSeconds),
      max_scroll_pct: maxScrollPct,
      sections: sections.map(function(s) {
        return {
          id: s.id,
          text: s.text,
          order: s.order,
          active_seconds: Math.round(s.active_seconds),
          viewed: s.viewed
        };
      }),
      completed: final
    };
    // Don't bother if nothing meaningful
    if (activeSeconds < 1 && !final) return;
    var body = JSON.stringify(payload);
    if (navigator.sendBeacon) {
      var blob = new Blob([body], { type: 'application/json' });
      navigator.sendBeacon('/api/engagement', blob);
    } else {
      // Fallback
      try {
        fetch('/api/engagement', { method: 'POST', body: body, keepalive: true, headers: { 'Content-Type': 'application/json' } });
      } catch (e) {}
    }
    flushed = true;
  }

  window.addEventListener('pagehide', function() { flush(true); });
  window.addEventListener('beforeunload', function() { flush(true); });

  // Init
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() {
      initSections();
      initObserver();
      updateScroll();
    });
  } else {
    initSections();
    initObserver();
    updateScroll();
  }

  // Start tick loop
  setInterval(tick, 500);
  window.addEventListener('scroll', updateScroll, { passive: true });
})();

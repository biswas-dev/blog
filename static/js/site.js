// Site-wide behaviour shared by every page (theme toggle, menus, search, relative dates,
// go-draw embed scroll guard). Loaded as a classic script at the end of <body>: pages call
// some of these globals (convertToRelativeTime, Search) from their own inline scripts.
function searchIcon(cls) {
    return '<svg class="' + cls + '" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg>';
}

        // Update year
        document.getElementById('current-year').textContent = new Date().getFullYear();

        // Theme toggle
        const themeToggle = document.getElementById('theme-toggle');
        const html = document.documentElement;

        themeToggle.addEventListener('click', () => {
            if (html.classList.contains('dark')) {
                html.classList.remove('dark');
                localStorage.setItem('theme', 'light');
            } else {
                html.classList.add('dark');
                localStorage.setItem('theme', 'dark');
            }
        });

        // Mobile menu toggle
        const mobileMenuBtn = document.getElementById('mobile-menu-btn');
        const mobileMenu = document.getElementById('mobile-menu');

        mobileMenuBtn.addEventListener('click', () => {
            mobileMenu.classList.toggle('open');
        });

        // Close mobile menu when clicking outside
        document.addEventListener('click', (e) => {
            if (!mobileMenuBtn.contains(e.target) && !mobileMenu.contains(e.target)) {
                mobileMenu.classList.remove('open');
            }
        });

        // Admin dropdown toggle
        const adminDropdown = document.querySelector('.admin-dropdown');
        const adminDropdownBtn = document.querySelector('.admin-dropdown-btn');

        if (adminDropdownBtn && adminDropdown) {
            const openMenu = () => adminDropdown.classList.add('open');
            const closeMenu = () => adminDropdown.classList.remove('open');

            adminDropdownBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                if (adminDropdown.classList.contains('open')) {
                    closeMenu();
                } else {
                    openMenu();
                }
            });

            document.addEventListener('click', (e) => {
                if (!adminDropdown.contains(e.target)) {
                    closeMenu();
                }
            });
        }

        // User dropdown toggle
        const userDropdown = document.querySelector('.user-dropdown');
        const userDropdownBtn = document.querySelector('.user-dropdown-btn');

        if (userDropdownBtn && userDropdown) {
            const openMenu = () => userDropdown.classList.add('open');
            const closeMenu = () => userDropdown.classList.remove('open');

            userDropdownBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                if (userDropdown.classList.contains('open')) {
                    closeMenu();
                } else {
                    openMenu();
                }
            });

            document.addEventListener('click', (e) => {
                if (!userDropdown.contains(e.target)) {
                    closeMenu();
                }
            });
        }

        // Search functionality
        const Search = {
            _activeIndex: -1,

            open() {
                const overlay = document.createElement('div');
                overlay.className = 'search-modal-overlay';
                overlay.innerHTML = `
                    <div class="search-modal">
                        <div class="search-modal-header">
                            ${searchIcon('search-modal-icon')}
                            <input type="text" placeholder="Search posts and slides..." class="search-modal-input" />
                            <kbd class="search-kbd">ESC</kbd>
                        </div>
                        <div class="search-results-container" id="search-results">
                            <div class="search-placeholder">
                                ${searchIcon('search-placeholder-icon')}
                                <p>Start typing to search posts and slides...</p>
                            </div>
                        </div>
                    </div>`;
                document.body.appendChild(overlay);

                const input = overlay.querySelector('input');
                setTimeout(() => input.focus(), 100);

                // Close on overlay click or Escape
                overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
                const escHandler = (e) => { if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', escHandler); } };
                document.addEventListener('keydown', escHandler);

                let timer, abort = null;
                this._activeIndex = -1;

                input.addEventListener('input', (e) => {
                    const q = e.target.value.trim();
                    clearTimeout(timer);
                    if (q.length < 2) { if (abort) abort.abort(); this._showPlaceholder(); this._activeIndex = -1; return; }
                    timer = setTimeout(() => { if (abort) abort.abort(); abort = new AbortController(); this._doSearch(q, abort.signal); }, 150);
                });

                input.addEventListener('keydown', (e) => {
                    const items = document.querySelectorAll('.search-result-item');
                    if (!items.length) return;
                    if (e.key === 'ArrowDown') { e.preventDefault(); this._activeIndex = Math.min(this._activeIndex + 1, items.length - 1); this._highlight(items); }
                    else if (e.key === 'ArrowUp') { e.preventDefault(); this._activeIndex = Math.max(this._activeIndex - 1, -1); this._highlight(items); }
                    else if (e.key === 'Enter' && this._activeIndex >= 0) { e.preventDefault(); items[this._activeIndex].click(); }
                });
            },

            _highlight(items) {
                items.forEach((el, i) => { el.classList.toggle('search-result-active', i === this._activeIndex); if (i === this._activeIndex) el.scrollIntoView({ block: 'nearest' }); });
            },

            async _doSearch(q, signal) {
                const c = document.getElementById('search-results');
                c.innerHTML = '<div class="search-loading"><div class="search-spinner"></div><p>Searching...</p></div>';
                try {
                    const r = await fetch('/api/search?q=' + encodeURIComponent(q), { signal });
                    const data = await r.json();
                    this._activeIndex = -1;
                    this._render(data);
                } catch (e) { if (e.name !== 'AbortError') { c.innerHTML = '<div class="search-error"><p>Search unavailable. Try again later.</p></div>'; } }
            },

            _esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; },

            _render(data) {
                const c = document.getElementById('search-results');
                if (!data || (data.posts.length === 0 && data.slides.length === 0)) {
                    c.innerHTML = '<div class="search-placeholder"><p>No results found for "' + this._esc(data ? data.query : '') + '"</p></div>';
                    return;
                }
                let h = '';
                if (data.posts.length > 0) {
                    h += '<div class="search-section-header">Posts</div>';
                    h += data.posts.map(r => this._item(r, 'post')).join('');
                }
                if (data.slides.length > 0) {
                    h += '<div class="search-section-header">Slides</div>';
                    h += data.slides.map(r => this._item(r, 'slide')).join('');
                }
                c.innerHTML = h;
            },

            _item(r, type) {
                const href = type === 'post' ? '/blog/' + r.slug : '/slides/' + r.slug;
                const cats = (r.categories || []).map(c => '<span class="search-category-tag">' + this._esc(c) + '</span>').join('');
                return '<a href="' + href + '" class="search-result-item">' +
                    '<div class="search-result-title">' + this._esc(r.title) + '</div>' +
                    '<div class="search-result-excerpt line-clamp-2">' + (r.excerpt || '') + '</div>' +
                    '<div class="search-result-meta"><span class="search-result-date">' + this._esc(r.date || '') + '</span>' +
                    (cats ? '<div class="search-result-categories">' + cats + '</div>' : '') +
                    '</div></a>';
            },

            _showPlaceholder() {
                const c = document.getElementById('search-results');
                if (c) c.innerHTML = '<div class="search-placeholder">' + searchIcon('search-placeholder-icon') + '<p>Start typing to search posts and slides...</p></div>';
            }
        };

        document.getElementById('search-toggle').addEventListener('click', () => {
            if (window.openKeynavSearch) { window.openKeynavSearch(); return; }
            Search.open();
        });

        // Ctrl/Cmd + K is handled by /static/js/keynav.js

        // DOMContentLoaded utilities
        document.addEventListener('DOMContentLoaded', () => {
            // Convert dates to relative time
            convertToRelativeTime();
            // Calculate reading times
            calculateReadingTimes();
        });

        function convertToRelativeTime() {
            document.querySelectorAll('.relative-time').forEach(element => {
                const isoDate = element.getAttribute('datetime');
                if (isoDate) {
                    element.textContent = getRelativeTime(new Date(isoDate));
                }
            });
        }

        function getRelativeTime(date) {
            const now = new Date();
            const diffInSeconds = Math.floor((now - date) / 1000);

            if (diffInSeconds < 60) return 'just now';
            if (diffInSeconds < 3600) {
                const m = Math.floor(diffInSeconds / 60);
                return `${m} ${m === 1 ? 'minute' : 'minutes'} ago`;
            }
            if (diffInSeconds < 86400) {
                const h = Math.floor(diffInSeconds / 3600);
                return `${h} ${h === 1 ? 'hour' : 'hours'} ago`;
            }
            if (diffInSeconds < 2592000) {
                const d = Math.floor(diffInSeconds / 86400);
                return `${d} ${d === 1 ? 'day' : 'days'} ago`;
            }
            if (diffInSeconds < 31536000) {
                const mo = Math.floor(diffInSeconds / 2592000);
                return `${mo} ${mo === 1 ? 'month' : 'months'} ago`;
            }
            const y = Math.floor(diffInSeconds / 31536000);
            return `${y} ${y === 1 ? 'year' : 'years'} ago`;
        }

        function calculateReadingTimes() {
            document.querySelectorAll('.read-time').forEach(element => {
                const content = element.getAttribute('data-content');
                if (content) {
                    const plainText = content.replace(/<[^>]*>/g, '');
                    const wordCount = plainText.trim().split(/\s+/).length;
                    const minutes = Math.ceil(wordCount / 200);
                    element.textContent = `${minutes} min read`;
                }
            });
        }

    document.addEventListener('DOMContentLoaded', () => {
      const drawHoverDelay = (parseFloat(document.body.dataset.drawHoverDelay) || 3) * 1000;
      // Touch devices (no hover capability) get tap-to-activate instead of a
      // hover countdown, since there's no mouse to dwell with.
      const isTouch = window.matchMedia && window.matchMedia('(hover: none), (pointer: coarse)').matches;

      document.querySelectorAll('.godraw-embed').forEach(el => {
        if (getComputedStyle(el).position === 'static') el.style.position = 'relative';

        if (isTouch) {
          // Overlay shield blocks the iframe until the user deliberately taps
          // to interact, so page-scroll isn't hijacked by a diagram the finger
          // happens to land on.
          const shield = document.createElement('div');
          shield.className = 'draw-tap-shield';
          shield.innerHTML = '<span class="draw-tap-pill">\u270B Tap to interact</span>';
          el.appendChild(shield);

          const closeBtn = document.createElement('button');
          closeBtn.className = 'draw-tap-close';
          closeBtn.type = 'button';
          closeBtn.setAttribute('aria-label', 'Stop interacting with diagram');
          closeBtn.innerHTML = '\u00D7';
          el.appendChild(closeBtn);

          shield.addEventListener('click', (e) => {
            e.preventDefault();
            el.classList.add('draw-active');
          });
          closeBtn.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            el.classList.remove('draw-active');
          });
          // Tapping anywhere outside the embed drops interaction mode.
          document.addEventListener('touchstart', (e) => {
            if (!el.classList.contains('draw-active')) return;
            if (!el.contains(e.target)) el.classList.remove('draw-active');
          }, { passive: true });
          return;
        }

        // Desktop: hover with countdown ring.
        const hint = document.createElement('div');
        hint.className = 'draw-hover-hint';
        hint.innerHTML =
          '<span class="draw-hover-ring">' +
            '<svg viewBox="0 0 20 20">' +
              '<circle class="draw-ring-bg" cx="10" cy="10" r="8"></circle>' +
              '<circle class="draw-ring-fg" cx="10" cy="10" r="8" stroke-dasharray="50.27" stroke-dashoffset="50.27"></circle>' +
            '</svg>' +
          '</span>' +
          '<span class="draw-hover-label">Hover for ' + (drawHoverDelay/1000).toFixed(1) + 's to interact</span>';
        el.appendChild(hint);
        const label = hint.querySelector('.draw-hover-label');
        const ringFg = hint.querySelector('.draw-ring-fg');
        const CIRC = 50.27; // 2 * PI * r (r=8)
        let rafId = null;
        let startTs = 0;

        function reset() {
          if (rafId) { cancelAnimationFrame(rafId); rafId = null; }
          startTs = 0;
          label.textContent = 'Hover for ' + (drawHoverDelay/1000).toFixed(1) + 's to interact';
          ringFg.setAttribute('stroke-dashoffset', CIRC);
        }

        function tick(now) {
          if (!startTs) startTs = now;
          const elapsed = now - startTs;
          const remaining = Math.max(0, drawHoverDelay - elapsed);
          label.textContent = 'Hover for ' + (remaining/1000).toFixed(1) + 's to interact';
          const pct = Math.min(1, elapsed / drawHoverDelay);
          ringFg.setAttribute('stroke-dashoffset', CIRC * (1 - pct));
          if (remaining <= 0) {
            el.classList.add('draw-active');
            return;
          }
          rafId = requestAnimationFrame(tick);
        }

        el.addEventListener('mouseenter', () => {
          if (el.classList.contains('draw-active')) return;
          reset();
          rafId = requestAnimationFrame(tick);
        });
        el.addEventListener('mouseleave', () => {
          reset();
          el.classList.remove('draw-active');
        });
      });
    });

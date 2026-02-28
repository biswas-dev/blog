// Modern Theme JavaScript
// Enhanced functionality for the modern blog theme

(function() {
    'use strict';
    
    // Theme utilities
    const Theme = {
        // Initialize theme
        init() {
            this.setupThemeToggle();
            this.setupMobileMenu();
            this.setupSearch();
            this.setupScrollEffects();
            this.setupAnimations();
            this.setupTooltips();
            this.setupCodeBlocks();
            this.setupImageLightbox();
        },
        
        // Theme toggle functionality
        setupThemeToggle() {
            const themeToggle = document.getElementById('theme-toggle');
            const html = document.documentElement;
            
            if (!themeToggle) return;
            
            // Set initial theme based on localStorage or system preference
            const savedTheme = localStorage.getItem('theme') || 
                              (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
            
            if (savedTheme === 'dark') {
                html.classList.add('dark');
            }
            
            themeToggle.addEventListener('click', () => {
                const isDark = html.classList.contains('dark');
                
                if (isDark) {
                    html.classList.remove('dark');
                    localStorage.setItem('theme', 'light');
                } else {
                    html.classList.add('dark');
                    localStorage.setItem('theme', 'dark');
                }
                
                // Animate the transition
                document.body.style.transition = 'background-color 0.3s ease, color 0.3s ease';
                setTimeout(() => {
                    document.body.style.transition = '';
                }, 300);
            });
            
            // Listen for system theme changes
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
                if (!localStorage.getItem('theme')) {
                    if (e.matches) {
                        html.classList.add('dark');
                    } else {
                        html.classList.remove('dark');
                    }
                }
            });
        },
        
        // Mobile menu functionality
        setupMobileMenu() {
            const mobileMenuBtn = document.getElementById('mobile-menu-btn');
            const mobileMenu = document.getElementById('mobile-menu');
            
            if (!mobileMenuBtn || !mobileMenu) return;
            
            mobileMenuBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                mobileMenu.classList.toggle('open');
                
                // Update button icon
                const icon = mobileMenuBtn.querySelector('svg');
                if (mobileMenu.classList.contains('open')) {
                    icon.innerHTML = '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>';
                } else {
                    icon.innerHTML = '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>';
                }
            });
            
            // Close mobile menu when clicking outside
            document.addEventListener('click', (e) => {
                if (!mobileMenuBtn.contains(e.target) && !mobileMenu.contains(e.target)) {
                    mobileMenu.classList.remove('open');
                    const icon = mobileMenuBtn.querySelector('svg');
                    icon.innerHTML = '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>';
                }
            });
            
            // Close mobile menu on escape key
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape' && mobileMenu.classList.contains('open')) {
                    mobileMenu.classList.remove('open');
                    const icon = mobileMenuBtn.querySelector('svg');
                    icon.innerHTML = '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>';
                }
            });
        },
        
        // Search functionality
        setupSearch() {
            const searchToggle = document.getElementById('search-toggle');
            
            if (!searchToggle) return;
            
            searchToggle.addEventListener('click', () => {
                this.showSearchModal();
            });
            
            // Keyboard shortcut for search (Ctrl/Cmd + K)
            document.addEventListener('keydown', (e) => {
                if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                    e.preventDefault();
                    this.showSearchModal();
                }
            });
        },
        
        // Show search modal
        showSearchModal() {
            const modal = this.createSearchModal();
            document.body.appendChild(modal);
            
            // Focus on input
            const input = modal.querySelector('input');
            setTimeout(() => input.focus(), 100);
            
            // Setup search functionality
            this.setupSearchInput(input, modal);
        },
        
        // Create search modal
        createSearchModal() {
            const modal = document.createElement('div');
            modal.className = 'search-modal-overlay';
            modal.innerHTML = `
                <div class="search-modal">
                    <div class="search-modal-header">
                        <svg class="search-modal-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                        <input
                            type="text"
                            placeholder="Search posts and slides..."
                            class="search-modal-input"
                        />
                        <kbd class="search-kbd">ESC</kbd>
                    </div>
                    <div class="search-results-container" id="search-results">
                        <div class="search-placeholder">
                            <svg class="search-placeholder-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                            </svg>
                            <p>Start typing to search posts and slides...</p>
                        </div>
                    </div>
                </div>
            `;

            // Close modal events
            modal.addEventListener('click', (e) => {
                if (e.target === modal) modal.remove();
            });

            document.addEventListener('keydown', function escHandler(e) {
                if (e.key === 'Escape') {
                    modal.remove();
                    document.removeEventListener('keydown', escHandler);
                }
            });

            return modal;
        },

        // Setup search input functionality
        setupSearchInput(input, modal) {
            let searchTimeout;
            let abortController = null;
            this._searchActiveIndex = -1;

            input.addEventListener('input', (e) => {
                const query = e.target.value.trim();

                clearTimeout(searchTimeout);

                if (query.length < 2) {
                    if (abortController) abortController.abort();
                    this.showSearchPlaceholder();
                    this._searchActiveIndex = -1;
                    return;
                }

                searchTimeout = setTimeout(() => {
                    if (abortController) abortController.abort();
                    abortController = new AbortController();
                    this.performSearch(query, abortController.signal);
                }, 150);
            });

            // Keyboard navigation
            input.addEventListener('keydown', (e) => {
                const items = document.querySelectorAll('.search-result-item');
                if (!items.length) return;

                if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    this._searchActiveIndex = Math.min(this._searchActiveIndex + 1, items.length - 1);
                    this._updateSearchActiveItem(items);
                } else if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    this._searchActiveIndex = Math.max(this._searchActiveIndex - 1, -1);
                    this._updateSearchActiveItem(items);
                } else if (e.key === 'Enter' && this._searchActiveIndex >= 0) {
                    e.preventDefault();
                    items[this._searchActiveIndex].click();
                }
            });
        },

        // Update active search result highlight
        _updateSearchActiveItem(items) {
            items.forEach((item, i) => {
                if (i === this._searchActiveIndex) {
                    item.classList.add('search-result-active');
                    item.scrollIntoView({ block: 'nearest' });
                } else {
                    item.classList.remove('search-result-active');
                }
            });
        },

        // Perform search
        async performSearch(query, signal) {
            const resultsContainer = document.getElementById('search-results');

            // Show loading
            resultsContainer.innerHTML = `
                <div class="search-loading">
                    <div class="search-spinner"></div>
                    <p>Searching...</p>
                </div>
            `;

            try {
                const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`, { signal });
                const results = await response.json();

                this._searchActiveIndex = -1;
                this.displaySearchResults(results);
            } catch (error) {
                if (error.name === 'AbortError') return;
                console.error('Search error:', error);
                this.showSearchError();
            }
        },

        // Escape HTML to prevent XSS (titles are escaped, excerpts use server <mark> tags)
        _escapeHTML(str) {
            const div = document.createElement('div');
            div.textContent = str;
            return div.innerHTML;
        },

        // Display search results grouped by type
        displaySearchResults(results) {
            const resultsContainer = document.getElementById('search-results');

            if (!results || (results.posts.length === 0 && results.slides.length === 0)) {
                resultsContainer.innerHTML = `
                    <div class="search-placeholder">
                        <svg class="search-placeholder-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9.172 16.172a4 4 0 015.656 0M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                        </svg>
                        <p>No results found for "${this._escapeHTML(results ? results.query : '')}"</p>
                    </div>
                `;
                return;
            }

            let html = '';

            // Posts section
            if (results.posts.length > 0) {
                html += `<div class="search-section-header">Posts</div>`;
                html += results.posts.map(r => this._renderSearchResult(r, 'post')).join('');
            }

            // Slides section
            if (results.slides.length > 0) {
                html += `<div class="search-section-header">Slides</div>`;
                html += results.slides.map(r => this._renderSearchResult(r, 'slide')).join('');
            }

            resultsContainer.innerHTML = html;
        },

        // Render a single search result item
        _renderSearchResult(result, type) {
            const href = type === 'post' ? `/blog/${result.slug}` : `/slides/${result.slug}`;
            const cats = (result.categories || [])
                .map(c => `<span class="search-category-tag">${this._escapeHTML(c)}</span>`)
                .join('');

            return `
                <a href="${href}" class="search-result-item">
                    <div class="search-result-title">${this._escapeHTML(result.title)}</div>
                    <div class="search-result-excerpt line-clamp-2">${result.excerpt}</div>
                    <div class="search-result-meta">
                        <span class="search-result-date">${this._escapeHTML(result.date)}</span>
                        ${cats ? `<div class="search-result-categories">${cats}</div>` : ''}
                    </div>
                </a>
            `;
        },

        // Show search placeholder
        showSearchPlaceholder() {
            const resultsContainer = document.getElementById('search-results');
            if (resultsContainer) {
                resultsContainer.innerHTML = `
                    <div class="search-placeholder">
                        <svg class="search-placeholder-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                        <p>Start typing to search posts and slides...</p>
                    </div>
                `;
            }
        },

        // Show search error
        showSearchError() {
            const resultsContainer = document.getElementById('search-results');
            if (resultsContainer) {
                resultsContainer.innerHTML = `
                    <div class="search-error">
                        <svg class="search-placeholder-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                        </svg>
                        <p>Search is temporarily unavailable. Please try again later.</p>
                    </div>
                `;
            }
        },
        
        // Setup scroll effects
        setupScrollEffects() {
            // Back to top button
            const backToTopBtn = document.getElementById('back-to-top');
            
            if (backToTopBtn) {
                window.addEventListener('scroll', () => {
                    if (window.scrollY > 300) {
                        backToTopBtn.classList.remove('opacity-0', 'invisible');
                        backToTopBtn.classList.add('opacity-100', 'visible');
                    } else {
                        backToTopBtn.classList.add('opacity-0', 'invisible');
                        backToTopBtn.classList.remove('opacity-100', 'visible');
                    }
                });
                
                backToTopBtn.addEventListener('click', () => {
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                });
            }
            
            // Navbar scroll effect
            const navbar = document.querySelector('.navbar');
            if (navbar) {
                let lastScrollY = window.scrollY;
                
                window.addEventListener('scroll', () => {
                    const currentScrollY = window.scrollY;
                    
                    if (currentScrollY > 100) {
                        navbar.classList.add('navbar-scrolled');
                    } else {
                        navbar.classList.remove('navbar-scrolled');
                    }
                    
                    lastScrollY = currentScrollY;
                });
            }
        },
        
        // Setup animations
        setupAnimations() {
            // Intersection Observer for fade-in animations
            const observerOptions = {
                threshold: 0.1,
                rootMargin: '0px 0px -50px 0px'
            };
            
            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        entry.target.classList.add('animate-fade-in');
                        observer.unobserve(entry.target);
                    }
                });
            }, observerOptions);
            
            // Observe elements for animation
            const animateElements = document.querySelectorAll('.blog-post, .hero, .admin-dashboard > *');
            animateElements.forEach(el => {
                observer.observe(el);
            });
        },
        
        // Setup tooltips
        setupTooltips() {
            const tooltipElements = document.querySelectorAll('[data-tooltip]');
            
            tooltipElements.forEach(element => {
                element.addEventListener('mouseenter', (e) => {
                    this.showTooltip(e.target, e.target.dataset.tooltip);
                });
                
                element.addEventListener('mouseleave', () => {
                    this.hideTooltip();
                });
            });
        },
        
        // Show tooltip
        showTooltip(element, text) {
            const tooltip = document.createElement('div');
            tooltip.className = 'tooltip absolute z-50 px-2 py-1 text-sm bg-gray-900 text-white rounded shadow-lg';
            tooltip.textContent = text;
            tooltip.id = 'tooltip';
            
            document.body.appendChild(tooltip);
            
            const rect = element.getBoundingClientRect();
            tooltip.style.top = `${rect.top - tooltip.offsetHeight - 5}px`;
            tooltip.style.left = `${rect.left + (rect.width - tooltip.offsetWidth) / 2}px`;
        },
        
        // Hide tooltip
        hideTooltip() {
            const tooltip = document.getElementById('tooltip');
            if (tooltip) {
                tooltip.remove();
            }
        },
        
        // Setup code blocks
        setupCodeBlocks() {
            const codeBlocks = document.querySelectorAll('pre code');
            
            codeBlocks.forEach(block => {
                const pre = block.parentElement;
                pre.classList.add('relative');
                
                // Add copy button
                const copyBtn = document.createElement('button');
                copyBtn.className = 'absolute top-2 right-2 px-2 py-1 text-xs bg-gray-700 text-white rounded opacity-0 hover:opacity-100 transition-opacity';
                copyBtn.textContent = 'Copy';
                
                copyBtn.addEventListener('click', () => {
                    navigator.clipboard.writeText(block.textContent);
                    copyBtn.textContent = 'Copied!';
                    setTimeout(() => {
                        copyBtn.textContent = 'Copy';
                    }, 2000);
                });
                
                pre.appendChild(copyBtn);
                
                // Show copy button on hover
                pre.addEventListener('mouseenter', () => {
                    copyBtn.classList.add('opacity-100');
                });
                
                pre.addEventListener('mouseleave', () => {
                    copyBtn.classList.remove('opacity-100');
                });
            });
        },
        
        // Setup image lightbox
        setupImageLightbox() {
            const images = document.querySelectorAll('article img');
            
            images.forEach(img => {
                img.style.cursor = 'pointer';
                img.addEventListener('click', () => {
                    this.showLightbox(img.src, img.alt);
                });
            });
        },
        
        // Show image lightbox
        showLightbox(src, alt) {
            const lightbox = document.createElement('div');
            lightbox.className = 'fixed inset-0 bg-black/90 z-50 flex items-center justify-center p-4';
            lightbox.innerHTML = `
                <div class="relative max-w-full max-h-full">
                    <img src="${src}" alt="${alt}" class="max-w-full max-h-full object-contain rounded-lg">
                    <button class="absolute top-4 right-4 text-white hover:text-gray-300 text-2xl">×</button>
                </div>
            `;
            
            document.body.appendChild(lightbox);
            
            // Close lightbox
            const closeBtn = lightbox.querySelector('button');
            closeBtn.addEventListener('click', () => lightbox.remove());
            
            lightbox.addEventListener('click', (e) => {
                if (e.target === lightbox) lightbox.remove();
            });
            
            document.addEventListener('keydown', function escHandler(e) {
                if (e.key === 'Escape') {
                    lightbox.remove();
                    document.removeEventListener('keydown', escHandler);
                }
            });
        }
    };
    
    // Initialize theme when DOM is loaded
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => Theme.init());
    } else {
        Theme.init();
    }
    
    // Update year in footer
    const yearElement = document.getElementById('current-year');
    if (yearElement) {
        yearElement.textContent = new Date().getFullYear();
    }
    
    // Add navbar scrolled effect styles
    const style = document.createElement('style');
    style.textContent = `
        .navbar-scrolled {
            backdrop-filter: blur(20px);
            background: rgba(255, 255, 255, 0.9);
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
        }
        
        .dark .navbar-scrolled {
            background: rgba(3, 7, 18, 0.9);
            box-shadow: 0 1px 3px rgba(255, 255, 255, 0.1);
        }
        
        .animate-fade-in {
            animation: fade-in 0.6s ease-out forwards;
        }
        
        @keyframes fade-in {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
    `;
    document.head.appendChild(style);
    
    // Make Theme available globally for debugging
    window.Theme = Theme;
})();
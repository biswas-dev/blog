/**
 * PDF Annotator — PDF.js-based viewer with annotation overlay.
 *
 * Renders a PDF inside a scrollable container, lets the user select text to
 * create highlights/notes, and persists annotations via the REST API.
 *
 * Dependencies:
 *   - PDF.js 4.x loaded from CDN (ESM build)
 *   - A container element with data-pdf-url and data-paper-id attributes
 *
 * Usage (from paper-editor.gohtml):
 *   const annotator = new PdfAnnotator(container, pdfUrl, paperID);
 *   annotator.loadPdf();
 */

/* ------------------------------------------------------------------ */
/*  Bootstrap: load PDF.js from CDN                                   */
/* ------------------------------------------------------------------ */

const PDFJS_CDN = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.0.379/pdf.min.mjs';
const PDFJS_WORKER_CDN = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.0.379/pdf.worker.min.mjs';

// Annotation highlight colours (50% opacity applied in CSS)
const ANNOTATION_COLORS = {
  yellow: '#fef08a',
  green:  '#bbf7d0',
  blue:   '#bfdbfe',
  pink:   '#fbcfe8',
  orange: '#fed7aa',
};

// Default scale for rendering
const DEFAULT_SCALE = 1.5;
const MIN_SCALE = 0.5;
const MAX_SCALE = 4.0;
const SCALE_STEP = 0.25;

/* ------------------------------------------------------------------ */
/*  PdfAnnotator class                                                */
/* ------------------------------------------------------------------ */

class PdfAnnotator {
  /**
   * @param {HTMLElement} container  - The #pdf-viewer-container element
   * @param {string}      pdfUrl    - URL to fetch the PDF binary
   * @param {string|number} paperID - Paper ID for the annotation API
   */
  constructor(container, pdfUrl, paperID) {
    this.container = container;
    this.pdfUrl = pdfUrl;
    this.paperID = paperID;

    // PDF.js state
    this.pdfDoc = null;
    this.currentPage = 1;
    this.totalPages = 0;
    this.scale = DEFAULT_SCALE;

    // Annotations loaded from server
    this.annotations = [];

    // Currently selected highlight colour (toolbar buttons set this)
    this.activeColor = 'yellow';

    // Whether annotation overlays are visible
    this.annotationsVisible = true;

    // References to toolbar elements (set by _bindToolbar)
    this._toolbar = {};

    // The scrollable pages wrapper
    this.pagesContainer = container.querySelector('#pdf-pages-container');

    // Dark-mode detection helper
    this.isDark = document.documentElement.classList.contains('dark');

    // Currently open popup (so we can close it before opening another)
    this._activePopup = null;

    // PDF.js library reference — set in loadPdf()
    this._pdfjsLib = null;
  }

  /* ================================================================ */
  /*  Public API                                                      */
  /* ================================================================ */

  /**
   * Load PDF.js, fetch the document, render all pages, and load
   * existing annotations.
   */
  async loadPdf() {
    try {
      // Dynamically import PDF.js ESM build
      const pdfjsLib = await import(PDFJS_CDN);
      pdfjsLib.GlobalWorkerOptions.workerSrc = PDFJS_WORKER_CDN;
      this._pdfjsLib = pdfjsLib;

      // Fetch and open the PDF document
      const loadingTask = pdfjsLib.getDocument(this.pdfUrl);
      this.pdfDoc = await loadingTask.promise;
      this.totalPages = this.pdfDoc.numPages;

      // Wire up toolbar controls
      this._bindToolbar();
      this._updatePageIndicator();

      // Render every page into the scrollable container
      for (let i = 1; i <= this.totalPages; i++) {
        await this.renderPage(i);
      }

      // Load and render existing annotations
      await this.loadAnnotations();
    } catch (err) {
      console.error('[PdfAnnotator] Failed to load PDF:', err);
      this.pagesContainer.innerHTML =
        '<p style="color:#ef4444;padding:2rem;">Failed to load PDF. ' + err.message + '</p>';
    }
  }

  /**
   * Render a single page into the pages container.
   * Creates a canvas for the PDF content, a text layer for selection,
   * and an overlay div for annotations.
   *
   * @param {number} pageNum - 1-based page number
   */
  async renderPage(pageNum) {
    const page = await this.pdfDoc.getPage(pageNum);
    const viewport = page.getViewport({ scale: this.scale });

    // Wrapper for this page
    const pageWrapper = document.createElement('div');
    pageWrapper.className = 'pdf-page-wrapper';
    pageWrapper.dataset.pageNum = pageNum;
    pageWrapper.style.cssText =
      'position:relative;margin:0 auto;background:#fff;box-shadow:0 2px 8px rgba(0,0,0,0.15);' +
      'width:' + Math.floor(viewport.width) + 'px;height:' + Math.floor(viewport.height) + 'px;';

    // Canvas for the PDF render
    const canvas = document.createElement('canvas');
    canvas.width = Math.floor(viewport.width * (window.devicePixelRatio || 1));
    canvas.height = Math.floor(viewport.height * (window.devicePixelRatio || 1));
    canvas.style.cssText = 'width:100%;height:100%;display:block;';
    const ctx = canvas.getContext('2d');
    ctx.scale(window.devicePixelRatio || 1, window.devicePixelRatio || 1);
    pageWrapper.appendChild(canvas);

    // Text layer (for selection)
    const textLayerDiv = document.createElement('div');
    textLayerDiv.className = 'pdf-text-layer';
    textLayerDiv.style.cssText =
      'position:absolute;top:0;left:0;width:100%;height:100%;overflow:hidden;opacity:0.25;line-height:1;';
    pageWrapper.appendChild(textLayerDiv);

    // Annotation overlay layer
    const overlayDiv = document.createElement('div');
    overlayDiv.className = 'pdf-annotation-overlay';
    overlayDiv.dataset.pageNum = pageNum;
    overlayDiv.style.cssText =
      'position:absolute;top:0;left:0;width:100%;height:100%;pointer-events:none;';
    pageWrapper.appendChild(overlayDiv);

    this.pagesContainer.appendChild(pageWrapper);

    // Render the PDF page onto the canvas
    await page.render({ canvasContext: ctx, viewport: viewport }).promise;

    // Render the text layer for text selection
    const textContent = await page.getTextContent();
    this._renderTextLayer(textLayerDiv, textContent, viewport, pageNum);
  }

  /**
   * Fetch all annotations for this paper from the API and render them.
   */
  async loadAnnotations() {
    try {
      const resp = await fetch('/api/papers/' + this.paperID + '/annotations');
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      this.annotations = await resp.json();
      if (!Array.isArray(this.annotations)) this.annotations = [];

      // Render each annotation on the appropriate page overlay
      this.annotations.forEach(ann => this.renderAnnotationOverlay(ann));
    } catch (err) {
      console.warn('[PdfAnnotator] Could not load annotations:', err);
    }
  }

  /**
   * Draw a single annotation highlight on the correct page overlay.
   *
   * @param {Object} annotation - Annotation object from the API
   */
  renderAnnotationOverlay(annotation) {
    const pageNum = annotation.page_number;
    const overlay = this.pagesContainer.querySelector(
      '.pdf-annotation-overlay[data-page-num="' + pageNum + '"]'
    );
    if (!overlay) return;

    const wrapper = overlay.closest('.pdf-page-wrapper');
    const wrapperW = wrapper ? wrapper.offsetWidth : 1;
    const wrapperH = wrapper ? wrapper.offsetHeight : 1;

    const color = ANNOTATION_COLORS[annotation.color] || ANNOTATION_COLORS.yellow;

    const rect = document.createElement('div');
    rect.className = 'pdf-annotation-rect';
    rect.dataset.annotationId = annotation.id;
    rect.style.cssText =
      'position:absolute;pointer-events:auto;cursor:pointer;border-radius:2px;' +
      'background:' + color + ';opacity:0.5;' +
      'left:' + (annotation.x * 100) + '%;' +
      'top:' + (annotation.y * 100) + '%;' +
      'width:' + (annotation.width * 100) + '%;' +
      'height:' + (annotation.height * 100) + '%;';

    // Hover tooltip
    rect.title = annotation.note || annotation.selected_text || '';

    // Click to edit the annotation
    rect.addEventListener('click', (e) => {
      e.stopPropagation();
      this._showEditPopup(annotation, rect);
    });

    overlay.appendChild(rect);
  }

  /**
   * Show a popup to create a new annotation from selected text.
   *
   * @param {string} selectedText  - The text the user highlighted
   * @param {Object} boundingBox   - {x, y, width, height} relative to page (0-1)
   * @param {number} pageNum       - 1-based page number
   */
  showAnnotationPopup(selectedText, boundingBox, pageNum) {
    this._closePopup();

    const popup = document.createElement('div');
    popup.className = 'pdf-annotation-popup';
    popup.style.cssText =
      'position:fixed;z-index:10000;background:#fff;border:1px solid #d1d5db;border-radius:8px;' +
      'padding:12px;box-shadow:0 4px 16px rgba(0,0,0,0.15);width:300px;font-size:13px;' +
      (this.isDark ? 'background:#1e293b;border-color:#334155;color:#e2e8f0;' : '');

    // Position near the cursor (centred horizontally on the page)
    const containerRect = this.container.getBoundingClientRect();
    popup.style.top = (containerRect.top + 60) + 'px';
    popup.style.left = (containerRect.left + containerRect.width / 2 - 150) + 'px';

    // Quote preview
    const quote = document.createElement('div');
    quote.style.cssText =
      'font-style:italic;border-left:3px solid #6366f1;padding-left:8px;margin-bottom:8px;' +
      'max-height:60px;overflow:hidden;color:#6b7280;font-size:12px;';
    quote.textContent = selectedText.substring(0, 200) + (selectedText.length > 200 ? '...' : '');
    popup.appendChild(quote);

    // Colour picker
    const colorRow = document.createElement('div');
    colorRow.style.cssText = 'display:flex;gap:6px;margin-bottom:8px;';
    let chosenColor = this.activeColor;
    Object.keys(ANNOTATION_COLORS).forEach(name => {
      const swatch = document.createElement('button');
      swatch.type = 'button';
      swatch.style.cssText =
        'width:24px;height:24px;border-radius:50%;border:2px solid ' +
        (name === chosenColor ? '#111' : '#ccc') + ';cursor:pointer;background:' +
        ANNOTATION_COLORS[name] + ';';
      swatch.title = name;
      swatch.addEventListener('click', () => {
        chosenColor = name;
        colorRow.querySelectorAll('button').forEach(b => {
          b.style.borderColor = '#ccc';
        });
        swatch.style.borderColor = '#111';
      });
      colorRow.appendChild(swatch);
    });
    popup.appendChild(colorRow);

    // Note textarea
    const noteInput = document.createElement('textarea');
    noteInput.placeholder = 'Add a note (optional)';
    noteInput.rows = 3;
    noteInput.style.cssText =
      'width:100%;border:1px solid #d1d5db;border-radius:6px;padding:6px 8px;font-size:13px;' +
      'resize:vertical;margin-bottom:8px;' +
      (this.isDark ? 'background:#0f172a;border-color:#334155;color:#e2e8f0;' : '');
    popup.appendChild(noteInput);

    // Public checkbox
    const publicLabel = document.createElement('label');
    publicLabel.style.cssText = 'display:flex;align-items:center;gap:6px;margin-bottom:10px;cursor:pointer;font-size:12px;';
    const publicCb = document.createElement('input');
    publicCb.type = 'checkbox';
    publicLabel.appendChild(publicCb);
    publicLabel.appendChild(document.createTextNode('Make public'));
    popup.appendChild(publicLabel);

    // Buttons row
    const btnRow = document.createElement('div');
    btnRow.style.cssText = 'display:flex;gap:8px;justify-content:flex-end;';

    const cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.style.cssText =
      'padding:6px 14px;border:1px solid #d1d5db;border-radius:6px;background:transparent;cursor:pointer;font-size:13px;' +
      (this.isDark ? 'border-color:#475569;color:#94a3b8;' : '');
    cancelBtn.addEventListener('click', () => this._closePopup());
    btnRow.appendChild(cancelBtn);

    const saveBtn = document.createElement('button');
    saveBtn.type = 'button';
    saveBtn.textContent = 'Save Annotation';
    saveBtn.style.cssText =
      'padding:6px 14px;border:none;border-radius:6px;background:#6366f1;color:#fff;cursor:pointer;font-size:13px;';
    saveBtn.addEventListener('click', async () => {
      saveBtn.disabled = true;
      saveBtn.textContent = 'Saving...';
      const data = {
        page_number: pageNum,
        selected_text: selectedText,
        note: noteInput.value,
        color: chosenColor,
        is_public: publicCb.checked,
        x: boundingBox.x,
        y: boundingBox.y,
        width: boundingBox.width,
        height: boundingBox.height,
      };
      const saved = await this.saveAnnotation(data);
      if (saved) {
        this.annotations.push(saved);
        this.renderAnnotationOverlay(saved);
      }
      this._closePopup();
    });
    btnRow.appendChild(saveBtn);

    popup.appendChild(btnRow);
    document.body.appendChild(popup);
    this._activePopup = popup;

    // Close popup when clicking outside
    setTimeout(() => {
      document.addEventListener('mousedown', this._outsideClickHandler = (e) => {
        if (!popup.contains(e.target)) this._closePopup();
      });
    }, 100);
  }

  /**
   * Save a new annotation via the API.
   *
   * @param {Object} data - Annotation payload
   * @returns {Object|null} The saved annotation, or null on failure
   */
  async saveAnnotation(data) {
    try {
      const resp = await fetch('/api/papers/' + this.paperID + '/annotations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      return await resp.json();
    } catch (err) {
      console.error('[PdfAnnotator] Save failed:', err);
      alert('Failed to save annotation: ' + err.message);
      return null;
    }
  }

  /**
   * Update an existing annotation.
   *
   * @param {number|string} id   - Annotation ID
   * @param {Object}        data - Fields to update
   * @returns {Object|null}
   */
  async updateAnnotation(id, data) {
    try {
      const resp = await fetch('/api/papers/annotations/' + id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      return await resp.json();
    } catch (err) {
      console.error('[PdfAnnotator] Update failed:', err);
      alert('Failed to update annotation: ' + err.message);
      return null;
    }
  }

  /**
   * Delete an annotation.
   *
   * @param {number|string} id - Annotation ID
   * @returns {boolean}
   */
  async deleteAnnotation(id) {
    try {
      const resp = await fetch('/api/papers/annotations/' + id, { method: 'DELETE' });
      if (!resp.ok) throw new Error('HTTP ' + resp.status);

      // Remove from local array
      this.annotations = this.annotations.filter(a => a.id !== id);

      // Remove from DOM
      const rect = this.pagesContainer.querySelector(
        '.pdf-annotation-rect[data-annotation-id="' + id + '"]'
      );
      if (rect) rect.remove();

      return true;
    } catch (err) {
      console.error('[PdfAnnotator] Delete failed:', err);
      alert('Failed to delete annotation: ' + err.message);
      return false;
    }
  }

  /* ================================================================ */
  /*  Private helpers                                                  */
  /* ================================================================ */

  /**
   * Render text content spans so the user can select text on the PDF.
   * Each span is positioned absolutely to match the PDF glyph positions.
   */
  _renderTextLayer(textLayerDiv, textContent, viewport, pageNum) {
    const textItems = textContent.items;

    textItems.forEach(item => {
      const tx = item.transform;
      // tx = [scaleX, skewY, skewX, scaleY, translateX, translateY]
      const fontSize = Math.sqrt(tx[2] * tx[2] + tx[3] * tx[3]);

      const span = document.createElement('span');
      span.textContent = item.str;
      span.style.cssText =
        'position:absolute;white-space:pre;transform-origin:0% 0%;' +
        'font-size:' + fontSize + 'px;' +
        'left:' + tx[4] + 'px;' +
        'top:' + (viewport.height - tx[5]) + 'px;' +
        'color:transparent;';

      textLayerDiv.appendChild(span);
    });

    // Listen for text selection on this text layer
    textLayerDiv.addEventListener('mouseup', (e) => {
      const selection = window.getSelection();
      const selectedText = selection ? selection.toString().trim() : '';
      if (!selectedText || selectedText.length < 2) return;

      // Compute a bounding box relative to the page wrapper (0-1 range)
      const range = selection.getRangeAt(0);
      const rects = range.getClientRects();
      if (!rects.length) return;

      const wrapper = textLayerDiv.closest('.pdf-page-wrapper');
      const wrapperRect = wrapper.getBoundingClientRect();

      // Union of all selection rects
      let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
      for (const r of rects) {
        minX = Math.min(minX, r.left - wrapperRect.left);
        minY = Math.min(minY, r.top - wrapperRect.top);
        maxX = Math.max(maxX, r.right - wrapperRect.left);
        maxY = Math.max(maxY, r.bottom - wrapperRect.top);
      }

      const boundingBox = {
        x: minX / wrapperRect.width,
        y: minY / wrapperRect.height,
        width: (maxX - minX) / wrapperRect.width,
        height: (maxY - minY) / wrapperRect.height,
      };

      this.showAnnotationPopup(selectedText, boundingBox, pageNum);
    });
  }

  /**
   * Bind toolbar buttons to zoom/page-nav/colour actions.
   */
  _bindToolbar() {
    const t = this._toolbar;
    t.prevPage       = document.getElementById('pdf-prev-page');
    t.nextPage       = document.getElementById('pdf-next-page');
    t.pageIndicator  = document.getElementById('pdf-page-indicator');
    t.zoomIn         = document.getElementById('pdf-zoom-in');
    t.zoomOut        = document.getElementById('pdf-zoom-out');
    t.zoomLevel      = document.getElementById('pdf-zoom-level');
    t.addNote        = document.getElementById('pdf-add-note');
    t.toggleAnns     = document.getElementById('pdf-toggle-annotations');

    // Page navigation — scrolls to the target page wrapper
    if (t.prevPage) {
      t.prevPage.addEventListener('click', () => {
        if (this.currentPage > 1) {
          this.currentPage--;
          this._scrollToPage(this.currentPage);
          this._updatePageIndicator();
        }
      });
    }
    if (t.nextPage) {
      t.nextPage.addEventListener('click', () => {
        if (this.currentPage < this.totalPages) {
          this.currentPage++;
          this._scrollToPage(this.currentPage);
          this._updatePageIndicator();
        }
      });
    }

    // Zoom controls — re-render all pages at the new scale
    if (t.zoomIn) {
      t.zoomIn.addEventListener('click', () => {
        if (this.scale < MAX_SCALE) {
          this.scale = Math.min(this.scale + SCALE_STEP, MAX_SCALE);
          this._rerender();
        }
      });
    }
    if (t.zoomOut) {
      t.zoomOut.addEventListener('click', () => {
        if (this.scale > MIN_SCALE) {
          this.scale = Math.max(this.scale - SCALE_STEP, MIN_SCALE);
          this._rerender();
        }
      });
    }

    // Colour buttons in the toolbar
    document.querySelectorAll('.pdf-color-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        this.activeColor = btn.dataset.color;
        // Highlight the active colour button
        document.querySelectorAll('.pdf-color-btn').forEach(b => {
          b.style.outline = 'none';
        });
        btn.style.outline = '2px solid #6366f1';
        btn.style.outlineOffset = '2px';
      });
    });

    // "Add Note" button: creates a free-standing note annotation at the
    // center of the currently visible page
    if (t.addNote) {
      t.addNote.addEventListener('click', () => {
        this.showAnnotationPopup(
          '',
          { x: 0.3, y: 0.3, width: 0.4, height: 0.05 },
          this.currentPage
        );
      });
    }

    // Toggle annotation visibility
    if (t.toggleAnns) {
      t.toggleAnns.addEventListener('click', () => {
        this.annotationsVisible = !this.annotationsVisible;
        this.pagesContainer.querySelectorAll('.pdf-annotation-overlay').forEach(el => {
          el.style.display = this.annotationsVisible ? '' : 'none';
        });
        t.toggleAnns.textContent = this.annotationsVisible
          ? 'Toggle Annotations'
          : 'Show Annotations';
      });
    }

    // Track current page on scroll
    if (this.pagesContainer) {
      this.pagesContainer.addEventListener('scroll', () => {
        this._updateCurrentPageFromScroll();
      });
    }
  }

  /**
   * Update the "Page X / Y" indicator and zoom level display.
   */
  _updatePageIndicator() {
    const t = this._toolbar;
    if (t.pageIndicator) {
      t.pageIndicator.textContent = 'Page ' + this.currentPage + ' / ' + this.totalPages;
    }
    if (t.zoomLevel) {
      t.zoomLevel.textContent = Math.round(this.scale * 100) + '%';
    }
  }

  /**
   * Scroll the pages container so that the given page is visible.
   */
  _scrollToPage(pageNum) {
    const wrapper = this.pagesContainer.querySelector(
      '.pdf-page-wrapper[data-page-num="' + pageNum + '"]'
    );
    if (wrapper) {
      wrapper.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }

  /**
   * Determine which page is currently most visible and update currentPage.
   */
  _updateCurrentPageFromScroll() {
    const wrappers = this.pagesContainer.querySelectorAll('.pdf-page-wrapper');
    const containerTop = this.pagesContainer.scrollTop;
    const containerMid = containerTop + this.pagesContainer.clientHeight / 2;

    let best = 1;
    wrappers.forEach(w => {
      const top = w.offsetTop;
      const bottom = top + w.offsetHeight;
      if (top <= containerMid && bottom >= containerMid) {
        best = parseInt(w.dataset.pageNum, 10);
      }
    });

    if (best !== this.currentPage) {
      this.currentPage = best;
      this._updatePageIndicator();
    }
  }

  /**
   * Re-render all pages (e.g. after zoom change). Clears the container and
   * re-renders from scratch.
   */
  async _rerender() {
    this.pagesContainer.innerHTML = '';
    for (let i = 1; i <= this.totalPages; i++) {
      await this.renderPage(i);
    }
    // Re-draw annotation overlays
    this.annotations.forEach(ann => this.renderAnnotationOverlay(ann));
    this._updatePageIndicator();
  }

  /**
   * Close any currently open popup.
   */
  _closePopup() {
    if (this._activePopup) {
      this._activePopup.remove();
      this._activePopup = null;
    }
    if (this._outsideClickHandler) {
      document.removeEventListener('mousedown', this._outsideClickHandler);
      this._outsideClickHandler = null;
    }
    // Clear text selection
    window.getSelection()?.removeAllRanges();
  }

  /**
   * Show an edit popup for an existing annotation (change colour, note,
   * public status, or delete).
   *
   * @param {Object}      annotation - The annotation object
   * @param {HTMLElement}  rectEl     - The highlight rect element
   */
  _showEditPopup(annotation, rectEl) {
    this._closePopup();

    const popup = document.createElement('div');
    popup.className = 'pdf-annotation-popup';
    popup.style.cssText =
      'position:fixed;z-index:10000;background:#fff;border:1px solid #d1d5db;border-radius:8px;' +
      'padding:12px;box-shadow:0 4px 16px rgba(0,0,0,0.15);width:300px;font-size:13px;' +
      (this.isDark ? 'background:#1e293b;border-color:#334155;color:#e2e8f0;' : '');

    // Position near the annotation rect
    const rRect = rectEl.getBoundingClientRect();
    popup.style.top = Math.min(rRect.bottom + 8, window.innerHeight - 320) + 'px';
    popup.style.left = Math.min(rRect.left, window.innerWidth - 320) + 'px';

    // Quote
    if (annotation.selected_text) {
      const quote = document.createElement('div');
      quote.style.cssText =
        'font-style:italic;border-left:3px solid #6366f1;padding-left:8px;margin-bottom:8px;' +
        'max-height:60px;overflow:hidden;color:#6b7280;font-size:12px;';
      quote.textContent = annotation.selected_text.substring(0, 200);
      popup.appendChild(quote);
    }

    // Colour picker
    const colorRow = document.createElement('div');
    colorRow.style.cssText = 'display:flex;gap:6px;margin-bottom:8px;';
    let chosenColor = annotation.color || 'yellow';
    Object.keys(ANNOTATION_COLORS).forEach(name => {
      const swatch = document.createElement('button');
      swatch.type = 'button';
      swatch.style.cssText =
        'width:24px;height:24px;border-radius:50%;border:2px solid ' +
        (name === chosenColor ? '#111' : '#ccc') + ';cursor:pointer;background:' +
        ANNOTATION_COLORS[name] + ';';
      swatch.title = name;
      swatch.addEventListener('click', () => {
        chosenColor = name;
        colorRow.querySelectorAll('button').forEach(b => b.style.borderColor = '#ccc');
        swatch.style.borderColor = '#111';
      });
      colorRow.appendChild(swatch);
    });
    popup.appendChild(colorRow);

    // Note textarea
    const noteInput = document.createElement('textarea');
    noteInput.rows = 3;
    noteInput.value = annotation.note || '';
    noteInput.placeholder = 'Add a note (optional)';
    noteInput.style.cssText =
      'width:100%;border:1px solid #d1d5db;border-radius:6px;padding:6px 8px;font-size:13px;' +
      'resize:vertical;margin-bottom:8px;' +
      (this.isDark ? 'background:#0f172a;border-color:#334155;color:#e2e8f0;' : '');
    popup.appendChild(noteInput);

    // Public checkbox
    const publicLabel = document.createElement('label');
    publicLabel.style.cssText = 'display:flex;align-items:center;gap:6px;margin-bottom:10px;cursor:pointer;font-size:12px;';
    const publicCb = document.createElement('input');
    publicCb.type = 'checkbox';
    publicCb.checked = !!annotation.is_public;
    publicLabel.appendChild(publicCb);
    publicLabel.appendChild(document.createTextNode('Make public'));
    popup.appendChild(publicLabel);

    // Buttons row
    const btnRow = document.createElement('div');
    btnRow.style.cssText = 'display:flex;gap:8px;justify-content:space-between;';

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.textContent = 'Delete';
    deleteBtn.style.cssText =
      'padding:6px 14px;border:1px solid #ef4444;border-radius:6px;background:transparent;' +
      'color:#ef4444;cursor:pointer;font-size:13px;';
    deleteBtn.addEventListener('click', async () => {
      if (!confirm('Delete this annotation?')) return;
      await this.deleteAnnotation(annotation.id);
      this._closePopup();
    });
    btnRow.appendChild(deleteBtn);

    const rightBtns = document.createElement('div');
    rightBtns.style.cssText = 'display:flex;gap:8px;';

    const cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.style.cssText =
      'padding:6px 14px;border:1px solid #d1d5db;border-radius:6px;background:transparent;cursor:pointer;font-size:13px;' +
      (this.isDark ? 'border-color:#475569;color:#94a3b8;' : '');
    cancelBtn.addEventListener('click', () => this._closePopup());
    rightBtns.appendChild(cancelBtn);

    const saveBtn = document.createElement('button');
    saveBtn.type = 'button';
    saveBtn.textContent = 'Update';
    saveBtn.style.cssText =
      'padding:6px 14px;border:none;border-radius:6px;background:#6366f1;color:#fff;cursor:pointer;font-size:13px;';
    saveBtn.addEventListener('click', async () => {
      saveBtn.disabled = true;
      saveBtn.textContent = 'Saving...';
      const data = {
        note: noteInput.value,
        color: chosenColor,
        is_public: publicCb.checked,
      };
      const updated = await this.updateAnnotation(annotation.id, data);
      if (updated) {
        // Update local state
        Object.assign(annotation, updated);
        // Update the rect colour
        const color = ANNOTATION_COLORS[updated.color] || ANNOTATION_COLORS.yellow;
        rectEl.style.background = color;
        rectEl.title = updated.note || updated.selected_text || '';
      }
      this._closePopup();
    });
    rightBtns.appendChild(saveBtn);
    btnRow.appendChild(rightBtns);

    popup.appendChild(btnRow);
    document.body.appendChild(popup);
    this._activePopup = popup;

    // Close on outside click
    setTimeout(() => {
      document.addEventListener('mousedown', this._outsideClickHandler = (e) => {
        if (!popup.contains(e.target)) this._closePopup();
      });
    }, 100);
  }
}

// Expose globally so the template can instantiate it
window.PdfAnnotator = PdfAnnotator;

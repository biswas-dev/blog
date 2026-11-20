package controllers

// Slide PDF export.
//
// Browser-exact PDF of a deck. We do NOT re-implement reveal.js layout — we
// drive a headless Chrome (the `pdfsvc` sidecar, a stock chromedp/headless-shell
// image) at the deck's own `?print-pdf` view and capture Page.PrintToPDF with
// preferCSSPageSize — the same technique decktape uses. This is the minimal
// "ripped" core of gopdfsuit's HTML-conversion path, without the rest of that
// suite (signatures, barcodes, forms, …).
//
// The blog app holds NO Chrome binary; it only speaks the DevTools Protocol
// (chromedp is pure Go) to the sidecar over the internal docker network. A
// short-lived HMAC export token lets the sidecar's Chrome fetch the one deck
// being exported past the password gate; the public endpoint enforces the same
// view rules first.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"anshumanbiswas.com/blog/models"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// pdfExportTTL bounds how long a minted export token (and the whole render) is
// valid. Generous enough for a cold Chrome tab + CDN asset fetch, short enough
// that a leaked URL is useless minutes later.
const pdfExportTTL = 90 * time.Second

// pdfRenderSem bounds concurrent Chrome tabs so a burst of export requests
// can't OOM the sidecar. Buffered to a small fixed fleet.
var pdfRenderSem = make(chan struct{}, 2)

// mintExportToken returns hex(HMAC-SHA256(secret, "slug|exp")).
func mintExportToken(secret, slug string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s|%d", slug, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// validExportToken constant-time-checks a token for slug and that exp is in the
// future. Returns false if the secret is unset (feature disabled / misconfig).
func validExportToken(secret, slug, token string, exp int64) bool {
	if secret == "" || token == "" {
		return false
	}
	if exp < time.Now().Unix() {
		return false
	}
	want := mintExportToken(secret, slug, exp)
	return hmac.Equal([]byte(want), []byte(token))
}

// hasValidExportToken reads export_token/exp from the request and validates them
// against this slug. Used by ViewSlide to let the sidecar's Chrome bypass the
// password gate and the published check for exactly the deck being exported.
func (s Slides) hasValidExportToken(r *http.Request, slug string) bool {
	q := r.URL.Query()
	exp, _ := strconv.ParseInt(q.Get("exp"), 10, 64)
	return validExportToken(s.ExportSecret, slug, q.Get("export_token"), exp)
}

// deckNoPDF answers PDF requests for DeckCraft decks: the reveal.js print path can't render
// them, and DeckCraft exports PDF itself (File › Export).
const deckNoPDF = "PDF export of DeckCraft decks is in the deck itself: File › Export"

// ExportSlidePDF handles GET /slides/{slug}/pdf — streams a browser-exact PDF.
func (s Slides) ExportSlidePDF(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if s.PDFRemoteURL == "" || s.ExportSecret == "" {
		http.Error(w, "PDF export is not configured", http.StatusServiceUnavailable)
		return
	}

	slide, err := s.SlideService.GetBySlug(slug)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}
	if slide.IsDeck {
		http.Error(w, deckNoPDF, http.StatusNotFound)
		return
	}

	// Access parity with ViewSlide: editors always; otherwise the deck must be
	// published, and if it is password-protected the caller must already hold a
	// granted access cookie (i.e. they got past the gate to view it).
	user, _ := s.isUserLoggedIn(r)
	isEditor := user != nil && models.CanEditSlides(user.Role)
	if !isEditor {
		if !slide.IsPublished {
			http.Error(w, "Slide not found", http.StatusNotFound)
			return
		}
		if slide.PasswordHash != "" {
			cookie, err := r.Cookie(fmt.Sprintf("slide_access_%s", slug))
			if err != nil || cookie.Value != "granted" {
				http.Error(w, "Unauthorized — open the presentation first", http.StatusUnauthorized)
				return
			}
		}
	}

	pdf, err := s.exportPDF(r.Context(), slug)
	if err != nil {
		log.Printf("slide pdf export (%s): %v", slug, err)
		http.Error(w, "Failed to render PDF", http.StatusBadGateway)
		return
	}
	writePDF(w, slug, pdf)
}

// APIExportSlidePDF handles GET/POST /api/slides/{slideID}/pdf — the
// Bearer-token-authenticated export path (the route group runs
// APIAuthMiddleware). For password-protected decks the caller must supply the
// correct deck password (query/form `password`), verified against the stored
// bcrypt hash, before we render — "only once the password has been provided".
func (s Slides) APIExportSlidePDF(w http.ResponseWriter, r *http.Request) {
	if s.PDFRemoteURL == "" || s.ExportSecret == "" {
		http.Error(w, "PDF export is not configured", http.StatusServiceUnavailable)
		return
	}

	slideID, err := strconv.Atoi(chi.URLParam(r, "slideID"))
	if err != nil {
		http.Error(w, "Invalid slide ID", http.StatusBadRequest)
		return
	}
	slide, err := s.SlideService.GetByID(slideID)
	if err != nil {
		http.Error(w, "Slide not found", http.StatusNotFound)
		return
	}
	if slide.IsDeck {
		http.Error(w, deckNoPDF, http.StatusNotFound)
		return
	}

	// Password-protected decks: require the deck password even for token
	// holders. Accept it from query (?password=) or form body.
	if slide.PasswordHash != "" {
		_ = r.ParseForm()
		password := r.URL.Query().Get("password")
		if password == "" {
			password = r.FormValue("password")
		}
		if password == "" {
			http.Error(w, "This deck is password-protected — provide the deck password", http.StatusUnauthorized)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(slide.PasswordHash), []byte(password)) != nil {
			http.Error(w, "Incorrect deck password", http.StatusForbidden)
			return
		}
	}

	pdf, err := s.exportPDF(r.Context(), slide.Slug)
	if err != nil {
		log.Printf("api slide pdf export (%s): %v", slide.Slug, err)
		http.Error(w, "Failed to render PDF", http.StatusBadGateway)
		return
	}
	writePDF(w, slide.Slug, pdf)
}

// exportPDF mints a short-lived token, builds the internal print URL the
// sidecar's Chrome will load, and renders it. ExportBaseURL is how that Chrome
// reaches THIS app over the docker network (e.g. http://app:22222), not the
// public hostname. Concurrency is bounded so export bursts can't OOM the
// sidecar.
func (s Slides) exportPDF(parent context.Context, slug string) ([]byte, error) {
	exp := time.Now().Add(pdfExportTTL).Unix()
	tok := mintExportToken(s.ExportSecret, slug, exp)
	printURL := fmt.Sprintf("%s/slides/%s?print-pdf&export_token=%s&exp=%d",
		strings.TrimRight(s.ExportBaseURL, "/"), url.PathEscape(slug), tok, exp)

	pdfRenderSem <- struct{}{}
	defer func() { <-pdfRenderSem }()

	ctx, cancel := context.WithTimeout(parent, pdfExportTTL)
	defer cancel()
	return renderDeckPDF(ctx, s.PDFRemoteURL, printURL)
}

// writePDF streams PDF bytes as a download.
func writePDF(w http.ResponseWriter, slug string, pdf []byte) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, slug))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

// renderDeckPDF connects to the remote Chrome (chromedp/headless-shell) at
// remoteBase (e.g. http://pdfsvc:9222), navigates to printURL, waits for
// reveal.js to finish its print-pdf layout, and returns the printed PDF bytes.
func renderDeckPDF(ctx context.Context, remoteBase, printURL string) ([]byte, error) {
	wsURL, err := resolveChromeWS(ctx, remoteBase)
	if err != nil {
		return nil, fmt.Errorf("resolve chrome ws: %w", err)
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer cancelAlloc()
	// Fresh tab per export; sharing the remote browser is fine.
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	var pdf []byte
	err = chromedp.Run(tabCtx,
		// A landscape-ish viewport so reveal lays the print pages out sensibly;
		// preferCSSPageSize below still honors reveal's own @page size.
		chromedp.EmulateViewport(1280, 720),
		chromedp.Navigate(printURL),
		// reveal.js sets document.title and toggles the print-pdf body class once
		// its print layout is ready; wait for that, then settle for fonts/CDN CSS.
		chromedp.ActionFunc(waitRevealPrintReady),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(ctx)
			if err != nil {
				return err
			}
			pdf = data
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	if len(pdf) == 0 {
		return nil, fmt.Errorf("empty pdf")
	}
	return pdf, nil
}

// waitRevealPrintReady polls until reveal.js has laid out its print-pdf pages
// (or a short deadline), injects a print-clip override, then settles for fonts.
//
// Reveal builds one .pdf-page per slide and stacks them; we wait for that
// structure rather than just Reveal.isReady() so we never print mid-layout.
//
// The injected override is essential: the slide template ships an inline
//
//	html, body { height: 100% !important; overflow: hidden !important }
//
// which (being !important and loaded after reveal's print stylesheet) clips the
// printed document to a single viewport, collapsing a 10-slide deck to ONE PDF
// page. We append a print-scoped override last so it wins the cascade and the
// full stacked height paginates.
func waitRevealPrintReady(ctx context.Context) error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var pages int
		// Prefer the print-layout signal (pdf-page count) over bare isReady().
		if err := chromedp.Evaluate(
			`document.querySelectorAll('.pdf-page').length`, &pages,
		).Do(ctx); err == nil && pages > 0 {
			break
		}
		if err := chromedp.Sleep(200 * time.Millisecond).Do(ctx); err != nil {
			return err
		}
	}
	// Override the template's print-clipping rule so the full deck paginates.
	var ignored interface{}
	_ = chromedp.Evaluate(`(function(){
	  var s = document.createElement('style');
	  s.setAttribute('data-pdf-export','1');
	  s.textContent = '@media print{html,body{height:auto !important;overflow:visible !important}.pdf-page:last-child{break-after:auto !important;page-break-after:auto !important}}';
	  document.head.appendChild(s);
	  return true;
	})()`, &ignored).Do(ctx)
	// Final settle for font swap + print stylesheet application.
	return chromedp.Sleep(1500 * time.Millisecond).Do(ctx)
}

// resolveChromeWS turns an http base (http://pdfsvc:9222) into the browser
// webSocketDebuggerUrl that NewRemoteAllocator needs.
//
// Critical detail: Chrome's DevTools endpoints (both the /json/* HTTP API and
// the WebSocket upgrade) reject any request whose Host header is not an IP
// address or "localhost" — a DNS-rebinding protection. The sidecar is reached
// by its compose service name (a hostname), which Chrome refuses. So we resolve
// the host to an IP up front and speak to Chrome purely by IP:port for both the
// version probe and the returned ws URL.
func resolveChromeWS(ctx context.Context, base string) (string, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "9222"
	}
	ipHostPort := net.JoinHostPort(host, port)
	if net.ParseIP(host) == nil && host != "localhost" {
		ips, lerr := net.DefaultResolver.LookupHost(ctx, host)
		if lerr != nil || len(ips) == 0 {
			return "", fmt.Errorf("resolve chrome host %q: %v", host, lerr)
		}
		ipHostPort = net.JoinHostPort(ips[0], port)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.Scheme+"://"+ipHostPort+"/json/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("no webSocketDebuggerUrl from %s: %s", ipHostPort, strings.TrimSpace(string(body)))
	}
	// Rewrite the ws host:port to the resolved IP so the upgrade request also
	// carries an IP Host header (and reaches the sidecar across the network).
	ws, err := url.Parse(v.WebSocketDebuggerURL)
	if err != nil {
		return v.WebSocketDebuggerURL, nil
	}
	ws.Host = ipHostPort
	return ws.String(), nil
}

package controllers

// Converts legacy go-slide decks to DeckCraft decks, once, on startup.
//
// Each legacy deck is opened in the headless-Chrome sidecar (as the PDF export does, with an export
// token past the password gate) and static/js/deck-convert.js turns every slide into a picture of
// everything except its text, with the text rebuilt on top as editable text boxes. The .pptx it
// builds becomes the deck's DeckCraft file. The original HTML stays in the row, so clearing
// deck_name brings the old deck back.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// deckConvertVersion goes on the converter's URL: .js is cached for a year, so a change to
// static/js/deck-convert.js must bump it or the sidecar Chrome keeps running the old one.
const deckConvertVersion = "6"

// convertTimeout bounds one deck: page load, every slide's capture and building the file.
const convertTimeout = 5 * time.Minute

type convertSlide struct {
	BG    string `json:"bg"`
	Boxes any    `json:"boxes"`
	Notes string `json:"notes"`
}

// ConvertLegacyDecks converts every deck that is not a DeckCraft deck yet. A deck that fails is
// logged and left as it is, and is tried again on the next start.
func (s Slides) ConvertLegacyDecks(ctx context.Context) {
	decks, err := s.SlideService.ListLegacyDecks()
	if err != nil {
		log.Printf("deck conversion: list: %v", err)
		return
	}
	if len(decks) == 0 {
		return
	}
	log.Printf("deck conversion: %d legacy deck(s) to convert", len(decks))
	for _, d := range decks {
		if ctx.Err() != nil {
			return
		}
		start := time.Now()
		data, count, err := s.convertDeck(ctx, d.Slug, d.Title)
		if err == nil {
			err = s.SlideService.SaveConvertedDeck(d.ID, d.Slug+".pptx", data, count)
		}
		if err != nil {
			log.Printf("deck conversion: %s (%d): %v", d.Slug, d.ID, err)
			continue
		}
		log.Printf("deck conversion: %s (%d): %d slides, %d bytes in %s", d.Slug, d.ID, count, len(data), time.Since(start).Round(time.Second))
	}
}

func (s Slides) convertDeck(parent context.Context, slug, title string) ([]byte, int, error) {
	exp := time.Now().Add(convertTimeout).Unix()
	deckURL := fmt.Sprintf("%s/slides/%s?export_token=%s&exp=%d", strings.TrimRight(s.ExportBaseURL, "/"),
		url.PathEscape(slug), mintExportToken(s.ExportSecret, slug, exp), exp)

	pdfRenderSem <- struct{}{}
	defer func() { <-pdfRenderSem }()
	ctx, cancel := context.WithTimeout(parent, convertTimeout)
	defer cancel()

	wsURL, err := resolveChromeWS(ctx, s.PDFRemoteURL)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve chrome ws: %w", err)
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer cancelAlloc()
	tab, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	var info struct {
		Count  int     `json:"count"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	err = chromedp.Run(tab,
		// The slide at 1:1, captured at twice the pixels so text-free pictures stay sharp.
		emulation.SetDeviceMetricsOverride(1280, 800, 2, false),
		chromedp.Navigate(deckURL),
		chromedp.WaitReady(".reveal .slides section", chromedp.ByQuery),
		chromedp.Evaluate(`new Promise((ok, fail) => {
			const load = (src) => new Promise((done, bad) => { const s = document.createElement('script'); s.src = src; s.onload = done; s.onerror = () => bad(src); document.head.appendChild(s); });
			load('/static/js/pptxgen-bundle.js').then(() => load('/static/js/deck-convert.js?v=`+deckConvertVersion+`')).then(() => document.fonts.ready).then(() => ok(true), fail);
		})`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }),
		chromedp.Evaluate(`deckConvert.prepare()`, &info),
		// Web fonts that finish loading after this would move text away from where it was measured.
		chromedp.Evaluate(`document.fonts.ready.then(() => new Promise((ok) => setTimeout(() => ok(true), 1000)))`, nil,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }),
	)
	if err != nil {
		return nil, 0, err
	}
	if info.Count == 0 {
		return nil, 0, fmt.Errorf("no slides")
	}

	for i := 0; i < info.Count; i++ {
		var rect struct{ X, Y, Width, Height float64 }
		var c struct {
			Boxes any    `json:"boxes"`
			Notes string `json:"notes"`
		}
		var bg []byte
		err := chromedp.Run(tab,
			chromedp.Evaluate(fmt.Sprintf(`deckConvert.show(%d)`, i), &rect),
			chromedp.Sleep(150*time.Millisecond),
			// Picture and text positions come from the same layout: hidden text keeps its place.
			chromedp.Evaluate(`deckConvert.hideText(true)`, nil),
			chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				bg, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).
					WithClip(&page.Viewport{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height, Scale: 1}).Do(ctx)
				return err
			}),
			chromedp.Evaluate(`deckConvert.collect()`, &c),
			chromedp.Evaluate(`deckConvert.hideText(false)`, nil),
		)
		if err != nil {
			return nil, 0, fmt.Errorf("slide %d: %w", i+1, err)
		}
		// Handed to the page one slide at a time, so no single message carries the whole deck.
		js, err := json.Marshal(convertSlide{BG: base64.StdEncoding.EncodeToString(bg), Boxes: c.Boxes, Notes: c.Notes})
		if err != nil {
			return nil, 0, err
		}
		if err := chromedp.Run(tab, chromedp.Evaluate(`deckConvert.keep(`+string(js)+`)`, nil)); err != nil {
			return nil, 0, fmt.Errorf("slide %d: %w", i+1, err)
		}
	}

	titleJS, _ := json.Marshal(title)
	var b64 string
	if err := chromedp.Run(tab, chromedp.Evaluate(`deckConvert.build(`+string(titleJS)+`)`, &b64,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) })); err != nil {
		return nil, 0, fmt.Errorf("build pptx: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, 0, err
	}
	if !strings.HasPrefix(string(data), string(zipMagic)) {
		return nil, 0, fmt.Errorf("built file is not a .pptx")
	}
	return data, info.Count, nil
}

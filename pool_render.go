package main

import (
	"context"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func (p *BrowserPool) Render(ctx context.Context, opts RenderOptions) (string, error) {
	var html string
	err := p.withPage(ctx, opts.TimeoutMS, opts.ViewportWidth, opts.ViewportHeight, opts.UserAgent, func(page *rod.Page) error {
		if err := p.navigateAndSettle(page, opts.URL, opts.WaitMS); err != nil {
			return err
		}
		var err error
		html, err = page.HTML()
		return err
	})
	return html, err
}

func (p *BrowserPool) Markdown(ctx context.Context, opts RenderOptions) (string, error) {
	var markdown string
	err := p.withPage(ctx, opts.TimeoutMS, opts.ViewportWidth, opts.ViewportHeight, opts.UserAgent, func(page *rod.Page) error {
		if err := p.navigateAndSettle(page, opts.URL, opts.WaitMS); err != nil {
			return err
		}

		extracted, err := extractMarkdown(page)
		if err != nil {
			return err
		}

		markdown = extracted
		return nil
	})
	return markdown, err
}

func (p *BrowserPool) Screenshot(ctx context.Context, opts ScreenshotOptions) (ScreenshotResult, error) {
	var result ScreenshotResult
	err := p.withPage(ctx, opts.TimeoutMS, opts.ViewportWidth, opts.ViewportHeight, opts.UserAgent, func(page *rod.Page) error {
		if err := p.navigateBasic(page, opts.URL, opts.WaitMS); err != nil {
			return err
		}

		format, contentType, protoFormat := normalizeFormat(opts.Format)
		quality := opts.Quality
		if quality < 1 {
			quality = 80
		}

		var qualityPtr *int
		if format != "png" {
			q := quality
			qualityPtr = &q
		}

		bytes, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
			Format:  protoFormat,
			Quality: qualityPtr,
		})
		if err != nil {
			return err
		}

		result = ScreenshotResult{
			Bytes:          bytes,
			ContentType:    contentType,
			Format:         format,
			ViewportWidth:  opts.ViewportWidth,
			ViewportHeight: opts.ViewportHeight,
		}
		return nil
	})
	return result, err
}

func normalizeFormat(format string) (string, string, proto.PageCaptureScreenshotFormat) {
	f := strings.ToLower(format)
	switch f {
	case "png":
		return "png", "image/png", proto.PageCaptureScreenshotFormatPng
	case "webp":
		return "webp", "image/webp", proto.PageCaptureScreenshotFormatWebp
	default:
		return "jpeg", "image/jpeg", proto.PageCaptureScreenshotFormatJpeg
	}
}

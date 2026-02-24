package main

import "strings"

type renderOptionInput struct {
	URL       string
	Viewport  *Viewport
	UserAgent string
	WaitMS    int
	TimeoutMS int
}

func resolveViewport(cfg Config, viewport *Viewport) (int, int) {
	viewportWidth := cfg.DefaultViewportWidth
	viewportHeight := cfg.DefaultViewportHeight
	if viewport != nil {
		if viewport.Width > 0 {
			viewportWidth = viewport.Width
		}
		if viewport.Height > 0 {
			viewportHeight = viewport.Height
		}
	}
	viewportWidth = clampMax(viewportWidth, cfg.MaxViewportWidth)
	viewportHeight = clampMax(viewportHeight, cfg.MaxViewportHeight)
	return viewportWidth, viewportHeight
}

func resolveUserAgent(cfg Config, userAgent string) string {
	if strings.TrimSpace(userAgent) == "" {
		return cfg.DefaultUserAgent
	}
	return strings.TrimSpace(userAgent)
}

func resolveWaitMS(cfg Config, waitMS int) int {
	value := waitMS
	if value <= 0 {
		value = int(cfg.DefaultWait.Milliseconds())
	}
	return clampMax(value, cfg.MaxWaitMS)
}

func resolveTimeoutMS(cfg Config, timeoutMS int) int {
	value := timeoutMS
	if value <= 0 {
		value = int(cfg.DefaultTimeout.Milliseconds())
	}
	return clampMax(value, cfg.MaxTimeoutMS)
}

func resolveDeviceScaleFactor(cfg Config, deviceScaleFactor float64) float64 {
	value := deviceScaleFactor
	if value <= 0 {
		value = cfg.DefaultDeviceScaleFactor
	}
	if value < 1 {
		value = 1
	}
	if cfg.MaxDeviceScaleFactor > 0 && value > cfg.MaxDeviceScaleFactor {
		value = cfg.MaxDeviceScaleFactor
	}
	return value
}

func resolveBaseRenderOptions(cfg Config, req renderOptionInput) RenderOptions {
	viewportWidth, viewportHeight := resolveViewport(cfg, req.Viewport)
	return RenderOptions{
		URL:            strings.TrimSpace(req.URL),
		ViewportWidth:  viewportWidth,
		ViewportHeight: viewportHeight,
		UserAgent:      resolveUserAgent(cfg, req.UserAgent),
		WaitMS:         resolveWaitMS(cfg, req.WaitMS),
		TimeoutMS:      resolveTimeoutMS(cfg, req.TimeoutMS),
	}
}

func resolveScreenshotOptions(cfg Config, req ScreenshotRequest) ScreenshotOptions {
	base := resolveBaseRenderOptions(cfg, renderOptionInput{
		URL:       req.URL,
		Viewport:  req.Viewport,
		UserAgent: req.UserAgent,
		WaitMS:    req.WaitMS,
		TimeoutMS: req.TimeoutMS,
	})

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = cfg.DefaultFormat
	}

	quality := req.Quality
	if quality <= 0 {
		quality = cfg.DefaultQuality
	}

	return ScreenshotOptions{
		URL:               base.URL,
		ViewportWidth:     base.ViewportWidth,
		ViewportHeight:    base.ViewportHeight,
		UserAgent:         base.UserAgent,
		WaitMS:            base.WaitMS,
		TimeoutMS:         base.TimeoutMS,
		Format:            format,
		Quality:           quality,
		DeviceScaleFactor: resolveDeviceScaleFactor(cfg, req.DeviceScaleFactor),
	}
}

func resolveRenderOptions(cfg Config, req RenderRequest) RenderOptions {
	return resolveBaseRenderOptions(cfg, renderOptionInput{
		URL:       req.URL,
		Viewport:  req.Viewport,
		UserAgent: req.UserAgent,
		WaitMS:    req.WaitMS,
		TimeoutMS: req.TimeoutMS,
	})
}

func resolveMarkdownOptions(cfg Config, req MarkdownRequest) RenderOptions {
	return resolveBaseRenderOptions(cfg, renderOptionInput{
		URL:       req.URL,
		Viewport:  req.Viewport,
		UserAgent: req.UserAgent,
		WaitMS:    req.WaitMS,
		TimeoutMS: req.TimeoutMS,
	})
}

func clampMax(value int, maxValue int) int {
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

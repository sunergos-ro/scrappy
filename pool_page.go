package main

import (
	"context"
	"errors"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

func newStealthPage(browser *rod.Browser) (*rod.Page, error) {
	var page *rod.Page
	if err := rod.Try(func() {
		page = stealth.MustPage(browser)
	}); err != nil {
		return nil, err
	}
	return page, nil
}

func runWithConfiguredPage(page *rod.Page, cfg Config, timeoutMS int, width int, height int, userAgent string, deviceScaleFactor float64, fn func(page *rod.Page) error) error {
	defer func() { _ = page.Close() }()

	if err := applyPageDefaults(page, width, height, userAgent, deviceScaleFactor); err != nil {
		return err
	}

	router, err := startRequestGuard(page, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = router.Stop() }()

	page = page.Timeout(time.Duration(timeoutMS) * time.Millisecond)
	return fn(page)
}

func startRequestGuard(page *rod.Page, cfg Config) (*rod.HijackRouter, error) {
	router := page.HijackRequests()
	if err := router.Add("*", "", func(h *rod.Hijack) {
		rawURL := ""
		if reqURL := h.Request.URL(); reqURL != nil {
			rawURL = reqURL.String()
		}
		if err := fetchRequestPolicy(cfg, rawURL, h.Request.IsNavigation()); err != nil {
			h.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
			return
		}
		h.ContinueRequest(&proto.FetchContinueRequest{})
	}); err != nil {
		_ = router.Stop()
		return nil, err
	}
	go router.Run()
	return router, nil
}

func (p *BrowserPool) withPage(ctx context.Context, timeoutMS int, width int, height int, userAgent string, deviceScaleFactor float64, fn func(page *rod.Page) error) error {
	if !p.cfg.PoolEnabled {
		return p.withStandalonePage(ctx, timeoutMS, width, height, userAgent, deviceScaleFactor, fn)
	}

	inst, err := p.checkout(p.cfg.PoolLeaseTimeout)
	if err != nil {
		if p.cfg.AllowStandaloneFallback {
			p.logger.Printf("pool checkout failed, falling back to standalone: %v", err)
			return p.withStandalonePage(ctx, timeoutMS, width, height, userAgent, deviceScaleFactor, fn)
		}
		p.logger.Printf("pool checkout failed: %v", err)
		return errors.New("browser pool unavailable")
	}

	page, err := newStealthPage(inst.Browser)
	if err != nil {
		p.markFailure(inst, err)
		p.release(inst)
		return err
	}

	err = runWithConfiguredPage(page, p.cfg, timeoutMS, width, height, userAgent, deviceScaleFactor, fn)

	if err != nil {
		p.markFailure(inst, err)
		p.release(inst)
		return err
	}

	p.markSuccess(inst)
	p.release(inst)
	return nil
}

func (p *BrowserPool) withStandalonePage(ctx context.Context, timeoutMS int, width int, height int, userAgent string, deviceScaleFactor float64, fn func(page *rod.Page) error) error {
	launched, err := p.launchBrowser()
	if err != nil {
		return err
	}
	p.trackStandaloneUserDataDir(launched.UserDataDir)
	defer p.untrackStandaloneUserDataDir(launched.UserDataDir)
	defer safeCloseBrowser(launched.Browser, launched.Launcher)

	page, err := newStealthPage(launched.Browser)
	if err != nil {
		return err
	}

	return runWithConfiguredPage(page, p.cfg, timeoutMS, width, height, userAgent, deviceScaleFactor, fn)
}

func applyPageDefaults(page *rod.Page, width int, height int, userAgent string, deviceScaleFactor float64) error {
	if userAgent != "" {
		if err := (proto.NetworkSetUserAgentOverride{UserAgent: userAgent}).Call(page); err != nil {
			return err
		}
	}

	scaleFactor := deviceScaleFactor
	if scaleFactor < 1 {
		scaleFactor = 1
	}

	if width > 0 && height > 0 {
		return (proto.EmulationSetDeviceMetricsOverride{
			Width:             width,
			Height:            height,
			DeviceScaleFactor: scaleFactor,
			Mobile:            false,
		}).Call(page)
	}
	return nil
}

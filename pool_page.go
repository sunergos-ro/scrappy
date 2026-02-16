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

func runWithConfiguredPage(page *rod.Page, timeoutMS int, width int, height int, userAgent string, fn func(page *rod.Page) error) error {
	defer func() { _ = page.Close() }()

	if err := applyPageDefaults(page, width, height, userAgent); err != nil {
		return err
	}

	page = page.Timeout(time.Duration(timeoutMS) * time.Millisecond)
	return fn(page)
}

func (p *BrowserPool) withPage(ctx context.Context, timeoutMS int, width int, height int, userAgent string, fn func(page *rod.Page) error) error {
	if !p.cfg.PoolEnabled {
		return p.withStandalonePage(ctx, timeoutMS, width, height, userAgent, fn)
	}

	inst, err := p.checkout(p.cfg.PoolLeaseTimeout)
	if err != nil {
		if p.cfg.AllowStandaloneFallback {
			p.logger.Printf("pool checkout failed, falling back to standalone: %v", err)
			return p.withStandalonePage(ctx, timeoutMS, width, height, userAgent, fn)
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

	err = runWithConfiguredPage(page, timeoutMS, width, height, userAgent, fn)

	if err != nil {
		p.markFailure(inst, err)
		p.release(inst)
		return err
	}

	p.markSuccess(inst)
	p.release(inst)
	return nil
}

func (p *BrowserPool) withStandalonePage(ctx context.Context, timeoutMS int, width int, height int, userAgent string, fn func(page *rod.Page) error) error {
	browser, err := p.launchBrowser()
	if err != nil {
		return err
	}
	defer func() { _ = browser.Close() }()

	page, err := newStealthPage(browser)
	if err != nil {
		return err
	}

	return runWithConfiguredPage(page, timeoutMS, width, height, userAgent, fn)
}

func applyPageDefaults(page *rod.Page, width int, height int, userAgent string) error {
	if userAgent != "" {
		if err := (proto.NetworkSetUserAgentOverride{UserAgent: userAgent}).Call(page); err != nil {
			return err
		}
	}

	if width > 0 && height > 0 {
		return (proto.EmulationSetDeviceMetricsOverride{
			Width:             width,
			Height:            height,
			DeviceScaleFactor: 1,
			Mobile:            false,
		}).Call(page)
	}
	return nil
}

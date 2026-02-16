package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func (p *BrowserPool) navigateBasic(page *rod.Page, url string, waitMS int) error {
	if err := page.Navigate(url); err != nil {
		return err
	}
	if err := page.WaitLoad(); err != nil {
		return err
	}

	if waitMS > 0 {
		if err := sleepWithContext(page.GetContext(), time.Duration(waitMS)*time.Millisecond); err != nil {
			return err
		}
	}

	if _, err := page.Eval(`async () => {
		if (document.fonts && document.fonts.ready) {
			await document.fonts.ready
		}
	}`); err != nil {
		p.logger.Printf("could not await fonts for %s: %v", url, err)
	}

	return nil
}

func (p *BrowserPool) navigateAndSettle(page *rod.Page, url string, waitMS int) error {
	if err := p.navigateBasic(page, url, waitMS); err != nil {
		return err
	}

	settleTimeout := defaultSettleTimeout
	if waitMS > 0 {
		waitDuration := time.Duration(waitMS) * time.Millisecond
		if waitDuration > settleTimeout {
			settleTimeout = waitDuration
		}
	}
	if settleTimeout > maxSettleTimeout {
		settleTimeout = maxSettleTimeout
	}

	p.waitForStableBestEffort(page, url, settleTimeout)

	textLen, err := bodyTextLength(page)
	if err != nil {
		p.logger.Printf("could not read body text length for %s: %v", url, err)
	} else if textLen < shortTextThreshold {
		retryWait := shortTextRetryWait
		if settleTimeout > retryWait {
			retryWait = settleTimeout
		}
		p.logger.Printf("short rendered body (%d chars) for %s, retrying settle", textLen, url)
		p.waitForStableBestEffort(page, url, retryWait)
	}

	return nil
}

func (p *BrowserPool) waitForStableBestEffort(page *rod.Page, url string, timeout time.Duration) {
	settledPage := page.Timeout(timeout)
	if err := settledPage.WaitStable(settleStableWindow); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return
		}
		p.logger.Printf("settle timeout for %s: %v", url, err)
	}
}

func bodyTextLength(page *rod.Page) (int, error) {
	obj, err := page.Eval(`() => {
		if (!document || !document.body) {
			return 0
		}
		const text = (document.body.innerText || "").replace(/\s+/g, " ").trim()
		return text.length
	}`)
	if err != nil {
		return 0, err
	}
	return obj.Value.Int(), nil
}

func extractMarkdown(page *rod.Page) (string, error) {
	obj, err := page.Eval(markdownExtractionScript)
	if err != nil {
		return "", err
	}

	markdown := normalizeMarkdownOutput(obj.Value.Str())
	if markdown != "" {
		return markdown, nil
	}

	textObj, err := page.Eval(bodyTextExtractionScript)
	if err != nil {
		return "", err
	}

	return normalizeMarkdownOutput(textObj.Value.Str()), nil
}

func normalizeMarkdownOutput(markdown string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(markdown, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	markdown = strings.Join(lines, "\n")
	markdown = strings.TrimSpace(markdown)

	for strings.Contains(markdown, "\n\n\n") {
		markdown = strings.ReplaceAll(markdown, "\n\n\n", "\n\n")
	}

	return markdown
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

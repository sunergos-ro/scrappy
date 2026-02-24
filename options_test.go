package main

import (
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		DefaultViewportWidth:     1200,
		DefaultViewportHeight:    700,
		DefaultUserAgent:         "default-agent",
		DefaultWait:              2500 * time.Millisecond,
		DefaultTimeout:           15000 * time.Millisecond,
		DefaultFormat:            "jpeg",
		DefaultQuality:           92,
		DefaultDeviceScaleFactor: 1.25,
		MaxWaitMS:                10000,
		MaxTimeoutMS:             30000,
		MaxViewportWidth:         2560,
		MaxViewportHeight:        2560,
		MaxDeviceScaleFactor:     3,
	}
}

func TestResolveRenderOptions(t *testing.T) {
	cfg := testConfig()

	tests := []struct {
		name string
		req  RenderRequest
		want RenderOptions
	}{
		{
			name: "uses defaults and trims url",
			req: RenderRequest{
				URL: "  https://example.com  ",
			},
			want: RenderOptions{
				URL:            "https://example.com",
				ViewportWidth:  1200,
				ViewportHeight: 700,
				UserAgent:      "default-agent",
				WaitMS:         2500,
				TimeoutMS:      15000,
			},
		},
		{
			name: "uses overrides",
			req: RenderRequest{
				URL:       "https://example.com",
				Viewport:  &Viewport{Width: 900, Height: 500},
				UserAgent: "  custom-agent  ",
				WaitMS:    5000,
				TimeoutMS: 12000,
			},
			want: RenderOptions{
				URL:            "https://example.com",
				ViewportWidth:  900,
				ViewportHeight: 500,
				UserAgent:      "custom-agent",
				WaitMS:         5000,
				TimeoutMS:      12000,
			},
		},
		{
			name: "uses partial viewport override",
			req: RenderRequest{
				URL:      "https://example.com",
				Viewport: &Viewport{Width: 1600},
			},
			want: RenderOptions{
				URL:            "https://example.com",
				ViewportWidth:  1600,
				ViewportHeight: 700,
				UserAgent:      "default-agent",
				WaitMS:         2500,
				TimeoutMS:      15000,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveRenderOptions(cfg, tc.req)
			if got != tc.want {
				t.Fatalf("resolveRenderOptions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestResolveScreenshotOptions(t *testing.T) {
	cfg := testConfig()

	tests := []struct {
		name string
		req  ScreenshotRequest
		want ScreenshotOptions
	}{
		{
			name: "uses screenshot defaults",
			req: ScreenshotRequest{
				URL: " https://example.com ",
			},
			want: ScreenshotOptions{
				URL:               "https://example.com",
				ViewportWidth:     1200,
				ViewportHeight:    700,
				UserAgent:         "default-agent",
				WaitMS:            2500,
				TimeoutMS:         15000,
				Format:            "jpeg",
				Quality:           92,
				DeviceScaleFactor: 1.25,
			},
		},
		{
			name: "uses screenshot overrides",
			req: ScreenshotRequest{
				URL:               "https://example.com",
				Viewport:          &Viewport{Width: 640, Height: 480},
				UserAgent:         " agent ",
				WaitMS:            800,
				TimeoutMS:         9000,
				Format:            " PNG ",
				Quality:           70,
				DeviceScaleFactor: 2,
			},
			want: ScreenshotOptions{
				URL:               "https://example.com",
				ViewportWidth:     640,
				ViewportHeight:    480,
				UserAgent:         "agent",
				WaitMS:            800,
				TimeoutMS:         9000,
				Format:            "png",
				Quality:           70,
				DeviceScaleFactor: 2,
			},
		},
		{
			name: "clamps screenshot device scale factor",
			req: ScreenshotRequest{
				URL:               "https://example.com",
				DeviceScaleFactor: 10,
			},
			want: ScreenshotOptions{
				URL:               "https://example.com",
				ViewportWidth:     1200,
				ViewportHeight:    700,
				UserAgent:         "default-agent",
				WaitMS:            2500,
				TimeoutMS:         15000,
				Format:            "jpeg",
				Quality:           92,
				DeviceScaleFactor: 3,
			},
		},
		{
			name: "normalizes screenshot device scale factor below one",
			req: ScreenshotRequest{
				URL:               "https://example.com",
				DeviceScaleFactor: 0.5,
			},
			want: ScreenshotOptions{
				URL:               "https://example.com",
				ViewportWidth:     1200,
				ViewportHeight:    700,
				UserAgent:         "default-agent",
				WaitMS:            2500,
				TimeoutMS:         15000,
				Format:            "jpeg",
				Quality:           92,
				DeviceScaleFactor: 1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveScreenshotOptions(cfg, tc.req)
			if got != tc.want {
				t.Fatalf("resolveScreenshotOptions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestResolveMarkdownOptions(t *testing.T) {
	cfg := testConfig()
	req := MarkdownRequest{
		URL:       " https://example.com ",
		Viewport:  &Viewport{Height: 900},
		UserAgent: " markdown-agent ",
		WaitMS:    3000,
	}

	got := resolveMarkdownOptions(cfg, req)
	want := RenderOptions{
		URL:            "https://example.com",
		ViewportWidth:  1200,
		ViewportHeight: 900,
		UserAgent:      "markdown-agent",
		WaitMS:         3000,
		TimeoutMS:      15000,
	}

	if got != want {
		t.Fatalf("resolveMarkdownOptions() = %+v, want %+v", got, want)
	}
}

func TestResolveRenderOptionsClampLimits(t *testing.T) {
	cfg := testConfig()
	cfg.MaxWaitMS = 2000
	cfg.MaxTimeoutMS = 8000
	cfg.MaxViewportWidth = 1400
	cfg.MaxViewportHeight = 900

	req := RenderRequest{
		URL:       "https://example.com",
		Viewport:  &Viewport{Width: 4000, Height: 3000},
		WaitMS:    10000,
		TimeoutMS: 20000,
	}

	got := resolveRenderOptions(cfg, req)
	want := RenderOptions{
		URL:            "https://example.com",
		ViewportWidth:  1400,
		ViewportHeight: 900,
		UserAgent:      "default-agent",
		WaitMS:         2000,
		TimeoutMS:      8000,
	}

	if got != want {
		t.Fatalf("resolveRenderOptions() = %+v, want %+v", got, want)
	}
}

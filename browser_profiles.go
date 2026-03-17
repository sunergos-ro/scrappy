package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func (p *BrowserPool) startProfileJanitor() {
	if p.cfg.ChromeProfileCleanupInterval <= 0 || p.cfg.ChromeProfileCleanupMaxAge <= 0 {
		return
	}

	go func() {
		p.cleanupStaleProfiles()

		ticker := time.NewTicker(p.cfg.ChromeProfileCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			p.mu.Lock()
			shutdown := p.shutdown
			p.mu.Unlock()
			if shutdown {
				return
			}
			p.cleanupStaleProfiles()
		}
	}()
}

func (p *BrowserPool) cleanupStaleProfiles() {
	active := p.activeUserDataDirsSnapshot()
	cutoff := time.Now().Add(-p.cfg.ChromeProfileCleanupMaxAge)
	removed, err := cleanupStaleUserDataDirs(p.cfg.ChromeUserDataDirRoot, active, cutoff)

	p.mu.Lock()
	p.lastProfileCleanup = time.Now()
	p.mu.Unlock()

	if err != nil {
		p.logger.Printf("profile cleanup failed: %v", err)
		return
	}
	if len(removed) > 0 {
		p.logEvent("profile_cleanup", fmt.Sprintf("Removed %d stale browser profile dirs", len(removed)), "", "info")
	}
}

func (p *BrowserPool) activeUserDataDirsSnapshot() map[string]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := make(map[string]struct{}, len(p.instances)+len(p.standaloneDirs))
	for _, inst := range p.instances {
		if inst.UserDataDir == "" {
			continue
		}
		active[filepath.Clean(inst.UserDataDir)] = struct{}{}
	}
	for dir := range p.standaloneDirs {
		active[filepath.Clean(dir)] = struct{}{}
	}
	return active
}

func (p *BrowserPool) trackStandaloneUserDataDir(dir string) {
	if dir == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.standaloneDirs[filepath.Clean(dir)] = struct{}{}
}

func (p *BrowserPool) untrackStandaloneUserDataDir(dir string) {
	if dir == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.standaloneDirs, filepath.Clean(dir))
}

func cleanupStaleUserDataDirs(root string, active map[string]struct{}, cutoff time.Time) ([]string, error) {
	if root == "" {
		return nil, errors.New("profile cleanup root is empty")
	}

	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	removed := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Clean(filepath.Join(root, entry.Name()))
		if _, ok := active[dir]; ok {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		if err := os.RemoveAll(dir); err != nil {
			return removed, err
		}
		removed = append(removed, dir)
	}

	return removed, nil
}

func safeCloseBrowser(browser *rod.Browser, browserLauncher *launcher.Launcher) {
	if browser != nil {
		done := make(chan struct{})
		go func() {
			_ = browser.Close()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if browserLauncher != nil {
				browserLauncher.Kill()
			}
		}
	}

	if browserLauncher == nil {
		return
	}

	cleanupDone := make(chan struct{})
	go func() {
		browserLauncher.Cleanup()
		close(cleanupDone)
	}()

	select {
	case <-cleanupDone:
		return
	case <-time.After(5 * time.Second):
		browserLauncher.Kill()
	}

	select {
	case <-cleanupDone:
	case <-time.After(5 * time.Second):
	}
}

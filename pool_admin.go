package main

import (
	"errors"
	"fmt"
	"log"
	"time"
)

func NewBrowserPool(cfg Config, logger *log.Logger) *BrowserPool {
	pool := &BrowserPool{
		cfg:            cfg,
		logger:         logger,
		instances:      make(map[string]*BrowserInstance),
		standaloneDirs: make(map[string]struct{}),
		desired:        cfg.PoolMinSize,
		events:         make([]PoolEvent, 0, maxEvents),
		utilization:    make([]PoolUtilization, 0, maxUtilizationPoints),
	}

	pool.startProfileJanitor()

	if cfg.PoolEnabled {
		go pool.supervisor()
	}

	return pool
}

func (p *BrowserPool) Preload() {
	if !p.cfg.PoolEnabled {
		return
	}
	p.ensureMinSize("preload")
}

func (p *BrowserPool) Shutdown() {
	p.mu.Lock()
	p.shutdown = true
	instances := make([]*BrowserInstance, 0, len(p.instances))
	for _, inst := range p.instances {
		instances = append(instances, inst)
	}
	p.instances = make(map[string]*BrowserInstance)
	p.mu.Unlock()

	for _, inst := range instances {
		p.safeQuit(inst)
	}
}

func (p *BrowserPool) Scale(size int) (map[string]any, error) {
	if !p.cfg.PoolEnabled {
		return nil, errors.New("browser pool disabled")
	}
	if size < 0 {
		return nil, errors.New("size must be >= 0")
	}
	if size > p.cfg.PoolMaxSize {
		return nil, fmt.Errorf("size exceeds max %d", p.cfg.PoolMaxSize)
	}

	p.mu.Lock()
	p.desired = size
	p.mu.Unlock()

	p.logEvent("scale", fmt.Sprintf("Scaling pool to %d", size), "", "info")
	p.ensureMinSize("manual-scale")
	p.trimExcess()
	return p.Stats(), nil
}

func (p *BrowserPool) Stats() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	counts := p.countByStatusLocked()
	instances := make([]map[string]any, 0, len(p.instances))
	for _, inst := range p.instances {
		instances = append(instances, map[string]any{
			"id":                inst.ID,
			"status":            inst.Status,
			"created_at":        inst.CreatedAt.Format(time.RFC3339),
			"last_used_at":      formatTime(inst.LastUsedAt),
			"lease_started_at":  formatTime(inst.LeaseStartedAt),
			"last_heartbeat_at": formatTime(inst.LastHeartbeatAt),
			"uses":              inst.Uses,
			"failures":          inst.Failures,
			"restarts":          inst.Restarts,
			"last_error":        inst.LastError,
		})
	}

	events := append([]PoolEvent(nil), p.events...)
	util := append([]PoolUtilization(nil), p.utilization...)

	return map[string]any{
		"config": map[string]any{
			"min_size":                        p.cfg.PoolMinSize,
			"max_size":                        p.cfg.PoolMaxSize,
			"desired_size":                    p.desired,
			"idle_ttl":                        p.cfg.PoolIdleTTL.Seconds(),
			"max_reuse":                       p.cfg.PoolMaxReuse,
			"lease_timeout":                   p.cfg.PoolLeaseTimeout.Seconds(),
			"spawn_timeout":                   p.cfg.PoolSpawnTimeout.Seconds(),
			"hang_timeout":                    p.cfg.PoolHangTimeout.Seconds(),
			"allow_standalone_fallback":       p.cfg.AllowStandaloneFallback,
			"chrome_no_sandbox_override":      p.cfg.ChromeNoSandbox,
			"chrome_user_data_dir_root":       p.cfg.ChromeUserDataDirRoot,
			"chrome_profile_cleanup_interval": p.cfg.ChromeProfileCleanupInterval.Seconds(),
			"chrome_profile_cleanup_max_age":  p.cfg.ChromeProfileCleanupMaxAge.Seconds(),
		},
		"counts":                  counts,
		"instances":               instances,
		"events":                  events,
		"utilization":             util,
		"last_profile_cleanup_at": formatTime(p.lastProfileCleanup),
		"last_supervisor_run_at":  formatTime(p.lastSupervisor),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

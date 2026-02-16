package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func (p *BrowserPool) checkout(timeout time.Duration) (*BrowserInstance, error) {
	deadline := time.Now().Add(timeout)

	for {
		p.mu.Lock()
		if p.shutdown {
			p.mu.Unlock()
			return nil, errors.New("pool shutting down")
		}

		inst := p.nextIdleLocked()
		if inst != nil {
			inst.Status = statusBusy
			inst.LeaseStartedAt = time.Now()
			inst.LastHeartbeatAt = time.Now()
			p.mu.Unlock()
			return inst, nil
		}

		total := len(p.instances)
		counts := p.countByStatusLocked()
		target := p.desired
		if target < p.cfg.PoolMinSize {
			target = p.cfg.PoolMinSize
		}

		healthyAvailable := counts[statusIdle] + counts[statusStarting]
		if healthyAvailable < target && total < p.cfg.PoolMaxSize {
			go p.spawn("lease")
		}

		p.mu.Unlock()

		if time.Now().After(deadline) {
			return nil, errors.New("timed out acquiring browser")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (p *BrowserPool) release(inst *BrowserInstance) {
	p.mu.Lock()
	defer p.mu.Unlock()

	inst.LastUsedAt = time.Now()
	inst.LeaseStartedAt = time.Time{}

	if inst.Uses >= p.cfg.PoolMaxReuse {
		p.removeInstanceLocked(inst.ID, "max_reuse")
		go p.ensureMinSize("reuse-rotate")
		return
	}

	if inst.Status != statusUnhealthy {
		inst.Status = statusIdle
	}
	p.recordUtilizationLocked()
}

func (p *BrowserPool) markSuccess(inst *BrowserInstance) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if inst.Status == statusBusy {
		inst.Status = statusIdle
	}
	inst.Uses++
}

func (p *BrowserPool) markFailure(inst *BrowserInstance, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	inst.Failures++
	inst.LastError = err.Error()
	inst.Status = statusUnhealthy
	p.logEventLocked("failure", fmt.Sprintf("Instance %s failed: %s", inst.ID, err.Error()), inst.ID, "warn")
	p.removeInstanceLocked(inst.ID, "failure")
}

func (p *BrowserPool) ensureMinSize(reason string) {
	if !p.cfg.PoolEnabled {
		return
	}

	p.mu.Lock()
	target := p.desired
	if target < p.cfg.PoolMinSize {
		target = p.cfg.PoolMinSize
	}
	available := p.countByStatusLocked()[statusIdle] + p.countByStatusLocked()[statusStarting]
	needed := target - available
	if needed < 0 {
		needed = 0
	}
	room := p.cfg.PoolMaxSize - len(p.instances)
	if needed > room {
		needed = room
	}
	p.mu.Unlock()

	for i := 0; i < needed; i++ {
		go p.spawn(reason)
	}
}

func (p *BrowserPool) trimExcess() {
	p.mu.Lock()
	target := p.desired
	if target < p.cfg.PoolMinSize {
		target = p.cfg.PoolMinSize
	}

	var idleIDs []string
	for id, inst := range p.instances {
		if inst.Status == statusIdle {
			idleIDs = append(idleIDs, id)
		}
	}

	excess := len(idleIDs) - target
	if excess <= 0 {
		p.mu.Unlock()
		return
	}

	toRemove := idleIDs[:excess]
	p.mu.Unlock()

	for _, id := range toRemove {
		p.removeInstance(id, "scale-down")
	}
}

func (p *BrowserPool) reapIdle() {
	p.mu.Lock()
	if len(p.instances) <= p.cfg.PoolMinSize {
		p.mu.Unlock()
		return
	}

	cutoff := time.Now().Add(-p.cfg.PoolIdleTTL)
	var candidates []string
	for id, inst := range p.instances {
		if inst.Status == statusIdle && !inst.LastUsedAt.IsZero() && inst.LastUsedAt.Before(cutoff) {
			candidates = append(candidates, id)
		}
	}
	p.mu.Unlock()

	for _, id := range candidates {
		p.removeInstance(id, "idle-ttl")
	}
}

func (p *BrowserPool) detectHangs() {
	p.mu.Lock()
	cutoff := time.Now().Add(-p.cfg.PoolHangTimeout)
	var hung []string
	for id, inst := range p.instances {
		if inst.Status == statusBusy && !inst.LeaseStartedAt.IsZero() && inst.LeaseStartedAt.Before(cutoff) {
			hung = append(hung, id)
		}
	}
	p.mu.Unlock()

	for _, id := range hung {
		p.logEvent("hang", "Lease exceeded hang timeout, recycling", id, "warn")
		p.removeInstance(id, "hang-timeout")
	}
}

func (p *BrowserPool) supervisor() {
	for {
		time.Sleep(p.cfg.PoolSupervisorInterval)
		if p.shutdown {
			return
		}
		p.mu.Lock()
		p.lastSupervisor = time.Now()
		p.mu.Unlock()

		p.ensureMinSize("supervisor")
		p.trimExcess()
		p.reapIdle()
		p.detectHangs()
	}
}

func (p *BrowserPool) spawn(reason string) {
	if !p.cfg.PoolEnabled {
		return
	}

	p.mu.Lock()
	if p.shutdown {
		p.mu.Unlock()
		return
	}

	id := newID()
	inst := &BrowserInstance{
		ID:              id,
		Status:          statusStarting,
		CreatedAt:       time.Now(),
		LaunchStartedAt: time.Now(),
		LastHeartbeatAt: time.Now(),
	}
	p.instances[id] = inst
	p.recordUtilizationLocked()
	p.mu.Unlock()

	browser, err := p.launchBrowser()
	if err != nil {
		p.mu.Lock()
		delete(p.instances, id)
		p.mu.Unlock()
		p.logEvent("spawn_failed", fmt.Sprintf("Spawn failed: %v", err), id, "error")
		return
	}

	p.mu.Lock()
	inst.Browser = browser
	inst.Status = statusIdle
	inst.LaunchStartedAt = time.Time{}
	p.recordUtilizationLocked()
	p.mu.Unlock()
	p.logEvent("spawn", fmt.Sprintf("Browser ready (reason: %s)", reason), id, "info")
}

func (p *BrowserPool) launchBrowser() (*rod.Browser, error) {
	launcherInstance := launcher.New().
		Leakless(true).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("headless", "new")

	if p.cfg.ChromeNoSandbox {
		launcherInstance = launcherInstance.Set("no-sandbox").Set("disable-setuid-sandbox")
	}

	if p.cfg.ChromeBin != "" {
		launcherInstance = launcherInstance.Bin(p.cfg.ChromeBin)
	}

	url, err := launcherInstance.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, err
	}

	return browser, nil
}

func (p *BrowserPool) removeInstance(id string, reason string) {
	p.mu.Lock()
	inst := p.instances[id]
	delete(p.instances, id)
	p.recordUtilizationLocked()
	p.mu.Unlock()
	if inst != nil {
		p.safeQuit(inst)
		p.logEvent("destroy", fmt.Sprintf("Removed instance (reason: %s)", reason), id, "info")
	}
}

func (p *BrowserPool) removeInstanceLocked(id string, reason string) {
	inst := p.instances[id]
	delete(p.instances, id)
	p.recordUtilizationLocked()
	if inst != nil {
		go p.safeQuit(inst)
		p.logEventLocked("destroy", fmt.Sprintf("Removed instance (reason: %s)", reason), id, "info")
	}
}

func (p *BrowserPool) safeQuit(inst *BrowserInstance) {
	if inst == nil || inst.Browser == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = inst.Browser.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func (p *BrowserPool) nextIdleLocked() *BrowserInstance {
	for _, inst := range p.instances {
		if inst.Status == statusIdle && inst.Browser != nil {
			return inst
		}
	}
	return nil
}

func (p *BrowserPool) countByStatusLocked() map[InstanceStatus]int {
	counts := map[InstanceStatus]int{
		statusIdle:      0,
		statusBusy:      0,
		statusStarting:  0,
		statusUnhealthy: 0,
	}
	for _, inst := range p.instances {
		counts[inst.Status]++
	}
	return counts
}

func (p *BrowserPool) logEvent(kind string, message string, instanceID string, level string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logEventLocked(kind, message, instanceID, level)
}

func (p *BrowserPool) logEventLocked(kind string, message string, instanceID string, level string) {
	event := PoolEvent{
		ID:         newID(),
		Kind:       kind,
		Message:    message,
		InstanceID: instanceID,
		At:         time.Now(),
		Level:      level,
	}
	p.events = append(p.events, event)
	if len(p.events) > maxEvents {
		p.events = p.events[1:]
	}
	p.logger.Printf("pool %s: %s", kind, message)
}

func (p *BrowserPool) recordUtilizationLocked() {
	counts := p.countByStatusLocked()
	p.utilization = append(p.utilization, PoolUtilization{
		At:        time.Now(),
		Total:     len(p.instances),
		Idle:      counts[statusIdle],
		Busy:      counts[statusBusy],
		Starting:  counts[statusStarting],
		Unhealthy: counts[statusUnhealthy],
	})
	if len(p.utilization) > maxUtilizationPoints {
		p.utilization = p.utilization[1:]
	}
}

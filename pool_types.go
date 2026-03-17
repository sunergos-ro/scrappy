package main

import (
	"log"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type InstanceStatus string

const (
	statusStarting  InstanceStatus = "starting"
	statusIdle      InstanceStatus = "idle"
	statusBusy      InstanceStatus = "busy"
	statusUnhealthy InstanceStatus = "unhealthy"
)

type BrowserInstance struct {
	ID              string
	Browser         *rod.Browser
	Launcher        *launcher.Launcher
	UserDataDir     string
	Status          InstanceStatus
	CreatedAt       time.Time
	LaunchStartedAt time.Time
	LastUsedAt      time.Time
	LeaseStartedAt  time.Time
	LastHeartbeatAt time.Time
	Uses            int
	Failures        int
	Restarts        int
	LastError       string
}

type BrowserPool struct {
	cfg                Config
	logger             *log.Logger
	mu                 sync.Mutex
	instances          map[string]*BrowserInstance
	standaloneDirs     map[string]struct{}
	desired            int
	shutdown           bool
	events             []PoolEvent
	utilization        []PoolUtilization
	lastSupervisor     time.Time
	lastProfileCleanup time.Time
}

type launchedBrowser struct {
	Browser     *rod.Browser
	Launcher    *launcher.Launcher
	UserDataDir string
}

type PoolEvent struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Message    string    `json:"message"`
	InstanceID string    `json:"instance_id,omitempty"`
	At         time.Time `json:"at"`
	Level      string    `json:"level"`
}

type PoolUtilization struct {
	At        time.Time `json:"at"`
	Total     int       `json:"total"`
	Idle      int       `json:"idle"`
	Busy      int       `json:"busy"`
	Starting  int       `json:"starting"`
	Unhealthy int       `json:"unhealthy"`
}

const (
	maxEvents            = 200
	maxUtilizationPoints = 300
	defaultSettleTimeout = 3 * time.Second
	maxSettleTimeout     = 10 * time.Second
	settleStableWindow   = 900 * time.Millisecond
	shortTextThreshold   = 300
	shortTextRetryWait   = 6 * time.Second
)

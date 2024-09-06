package config

import (
	"time"
)

type Log struct {
	Level            string
	Path             string
	RotationCount    uint
	RotationTime     time.Duration
	RotationSize     uint
	Compress         bool
	ReservedDiskSize uint
}

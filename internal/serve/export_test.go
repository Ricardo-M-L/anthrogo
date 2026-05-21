package serve

// This file exports internal types/functions for use in serve_test package tests.

import (
	"github.com/ricardo/anthrogo/pkg/query"
)

// ExportedSessionCache wraps sessionCache for white-box testing.
type ExportedSessionCache struct {
	c *sessionCache
}

// NewExportedSessionCache creates a new session cache for testing.
func NewExportedSessionCache() *ExportedSessionCache {
	return &ExportedSessionCache{c: newSessionCache()}
}

// GetOrCreate delegates to sessionCache.GetOrCreate, using query.Engine.
func (e *ExportedSessionCache) GetOrCreate(id string, builder func() (*query.Engine, error)) (*query.Engine, error) {
	return e.c.GetOrCreate(id, builder)
}

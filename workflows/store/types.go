// Package store provides a type-safe key-value store with advanced features
package store

import (
	"errors"
	"reflect"
	"time"
)

// Entry holds the serialized value plus its concrete Go type.
type entry struct {
	typ       reflect.Type
	blob      []byte
	expiresAt *time.Time // nil means no expiration
}

// MergeStrategy determines how key collisions are handled during a merge.
type MergeStrategy int

const (
	// Skip keeps the original value in case of collision
	Skip MergeStrategy = iota
	// Overwrite replaces the original value with the new one in case of collision
	Overwrite
	// Error fails the merge if a collision is detected
	Error
)

// Common errors returned by the store
var (
	ErrNotFound     = errors.New("key not found")
	ErrTypeMismatch = errors.New("type mismatch on Get")
	ErrExpired      = errors.New("key has expired")
)

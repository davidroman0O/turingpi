package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/morrisxyang/xreflect"
)

// KVStore is a threadsafe, type‑aware in‑memory store.
type KVStore struct {
	mu   sync.RWMutex
	data map[string]entry
}

// NewKVStore constructs an empty store.
func NewKVStore() *KVStore {
	return &KVStore{data: make(map[string]entry)}
}

// Put stores any Go value under key, capturing its concrete type.
func (s *KVStore) Put(key string, value any) error {
	return s.PutWithTTL(key, value, 0)
}

// PutWithTTL stores any Go value under key with a specified time-to-live duration.
// If ttl is 0 or negative, the entry will not expire.
func (s *KVStore) PutWithTTL(key string, value any, ttl time.Duration) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	blob, err := json.Marshal(value)
	if err != nil {
		return err
	}

	t := reflect.TypeOf(value)

	var expiresAt *time.Time
	if ttl > 0 {
		exp := time.Now().Add(ttl)
		expiresAt = &exp
	}

	s.mu.Lock()
	s.data[key] = entry{typ: t, blob: blob, expiresAt: expiresAt}
	s.mu.Unlock()
	return nil
}

// Get retrieves and unmarshals key into a value of type T.
func Get[T any](s *KVStore, key string) (T, error) {
	var zero T
	if key == "" {
		return zero, errors.New("key cannot be empty")
	}

	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return zero, ErrNotFound
	}

	// Check if the entry has expired
	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		s.Delete(key)
		return zero, ErrExpired
	}

	want := reflect.TypeOf((*T)(nil)).Elem()
	if e.typ != want {
		return zero, fmt.Errorf("%w: wanted %v, got %v",
			ErrTypeMismatch, want, e.typ)
	}

	var v T
	if err := json.Unmarshal(e.blob, &v); err != nil {
		return zero, err
	}

	return v, nil
}

// GetOrDefault retrieves a value of type T for the given key.
func GetOrDefault[T any](s *KVStore, key string, defaultValue T) (T, error) {
	value, err := Get[T](s, key)
	if err == ErrNotFound || err == ErrExpired {
		return defaultValue, nil
	}
	return value, err
}

// Delete removes a key from the store.
func (s *KVStore) Delete(key string) bool {
	if key == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.data[key]
	if exists {
		delete(s.data, key)
		return true
	}
	return false
}

// Clear removes all keys from the store.
func (s *KVStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]entry)
}

// ListKeys returns all stored keys.
func (s *KVStore) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]string, 0, len(s.data))
	for k, e := range s.data {
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// Count returns the number of valid entries in the store.
func (s *KVStore) Count() int {
	return len(s.ListKeys())
}

// ListTypes returns the set of all concrete types stored.
func (s *KVStore) ListTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[reflect.Type]struct{}{}
	out := []string{}

	for _, e := range s.data {
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		if _, ok := seen[e.typ]; ok {
			continue
		}
		seen[e.typ] = struct{}{}
		out = append(out, e.typ.String())
	}
	return out
}

// KeysByType returns all keys whose stored value has type T.
func KeysByType[T any](s *KVStore) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	want := reflect.TypeOf((*T)(nil)).Elem()
	keys := []string{}

	for k, e := range s.data {
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		if e.typ == want {
			keys = append(keys, k)
		}
	}
	return keys
}

// GetTypeSchema returns a JSON Schema representation of the stored value's type.
func (s *KVStore) GetTypeSchema(key string) (interface{}, error) {
	if key == "" {
		return nil, errors.New("key cannot be empty")
	}

	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrNotFound
	}

	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		s.Delete(key)
		return nil, ErrExpired
	}

	return TypeToSchema(e.typ), nil
}

// TypeToSchema converts a reflect.Type to a JSON schema.
func TypeToSchema(t reflect.Type) interface{} {
	instance := reflect.New(t).Interface()
	reflector := jsonschema.Reflector{
		ExpandedStruct: true,
	}
	return reflector.Reflect(instance)
}

// UpdateField updates a single field in a stored object using dot notation.
func (s *KVStore) UpdateField(key string, fieldPath string, fieldValue interface{}) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	if fieldPath == "" {
		return errors.New("fieldPath cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return ErrNotFound
	}

	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		delete(s.data, key)
		return ErrExpired
	}

	instance := reflect.New(e.typ).Interface()
	if err := json.Unmarshal(e.blob, instance); err != nil {
		return err
	}

	if err := xreflect.SetEmbedField(instance, fieldPath, fieldValue); err != nil {
		return fmt.Errorf("failed to update field: %w", err)
	}

	newBlob, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	s.data[key] = entry{
		typ:       e.typ,
		blob:      newBlob,
		expiresAt: e.expiresAt,
	}

	return nil
}

// UpdateFields updates multiple fields in a stored object.
func (s *KVStore) UpdateFields(key string, fields map[string]interface{}) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	if len(fields) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return ErrNotFound
	}

	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		delete(s.data, key)
		return ErrExpired
	}

	instance := reflect.New(e.typ).Interface()
	if err := json.Unmarshal(e.blob, instance); err != nil {
		return err
	}

	for fieldPath, fieldValue := range fields {
		if err := xreflect.SetEmbedField(instance, fieldPath, fieldValue); err != nil {
			return fmt.Errorf("failed to update field %s: %w", fieldPath, err)
		}
	}

	newBlob, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	s.data[key] = entry{
		typ:       e.typ,
		blob:      newBlob,
		expiresAt: e.expiresAt,
	}

	return nil
}

// Merge combines the contents of another KVStore into this one.
func (s *KVStore) Merge(other *KVStore, strategy MergeStrategy) ([]string, error) {
	collisions := s.FindKeyCollisions(other)

	if strategy == Error && len(collisions) > 0 {
		return collisions, fmt.Errorf("merge failed due to %d key collision(s)", len(collisions))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for k, otherEntry := range other.data {
		if otherEntry.expiresAt != nil && time.Now().After(*otherEntry.expiresAt) {
			continue
		}

		if _, exists := s.data[k]; exists {
			if strategy == Skip {
				continue
			}
		}

		s.data[k] = otherEntry
	}

	return collisions, nil
}

// FindKeyCollisions identifies keys that exist in both stores.
func (s *KVStore) FindKeyCollisions(other *KVStore) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	var collisions []string
	for k, e := range s.data {
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		if otherEntry, exists := other.data[k]; exists {
			if otherEntry.expiresAt != nil && time.Now().After(*otherEntry.expiresAt) {
				continue
			}
			collisions = append(collisions, k)
		}
	}

	return collisions
}

// FindKeysBySchema returns all keys whose type schema matches the given pattern.
// Pattern can be a partial schema - entries must contain at least all fields in pattern.
func (s *KVStore) FindKeysBySchema(pattern interface{}) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for k, e := range s.data {
		// Skip expired entries
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		schema := TypeToSchema(e.typ)
		if SchemaMatch(schema, pattern) {
			keys = append(keys, k)
		}
	}

	return keys
}

// SchemaMatch checks if a target schema matches a pattern schema.
// Pattern can be a partial schema - target must contain at least all fields in pattern.
func SchemaMatch(target, pattern interface{}) bool {
	// Convert to maps for easier comparison
	targetMap, targetOk := assertToMap(target)
	patternMap, patternOk := assertToMap(pattern)

	if !targetOk || !patternOk {
		return false
	}

	// Look for properties in the pattern
	if patternProps, ok := patternMap["properties"].(map[string]interface{}); ok {
		targetProps, ok := targetMap["properties"].(map[string]interface{})
		if !ok {
			return false
		}

		// All properties in pattern must exist in target
		for propName, propPattern := range patternProps {
			propTarget, exists := targetProps[propName]
			if !exists {
				return false
			}

			// If the property is an object, recursively check
			if propPatternMap, ok := assertToMap(propPattern); ok {
				// Only check if target can be converted to map
				if _, ok := assertToMap(propTarget); !ok {
					return false
				}

				// If it has properties, recurse
				if _, hasProps := propPatternMap["properties"]; hasProps {
					if !SchemaMatch(propTarget, propPattern) {
						return false
					}
				}
			}
		}
		return true
	}

	// If no properties, do a simple check on type
	if patternType, ok := patternMap["type"]; ok {
		targetType, ok := targetMap["type"]
		if !ok {
			return false
		}
		return patternType == targetType
	}

	// Default to true for empty pattern
	return true
}

// assertToMap tries to convert an interface to a map[string]interface{}
func assertToMap(v interface{}) (map[string]interface{}, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}

	// Try marshaling and unmarshaling if it's not already a map
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}

	return m, true
}

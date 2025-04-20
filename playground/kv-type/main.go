package main

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

// entry holds the serialized value plus its concrete Go type.
type entry struct {
	typ       reflect.Type
	blob      []byte
	expiresAt *time.Time // nil means no expiration
}

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

var (
	ErrNotFound     = errors.New("key not found")
	ErrTypeMismatch = errors.New("type mismatch on Get")
	ErrExpired      = errors.New("key has expired")
)

// Get retrieves and unmarshals key into a value of type T.
// It returns ErrNotFound if key is missing, ErrTypeMismatch if T doesn't match the stored type,
// and ErrExpired if the key has expired.
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
		// Delete the expired key
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
// If the key doesn't exist or has expired, it returns the provided default value.
// If there's a type mismatch, it still returns an error.
func GetOrDefault[T any](s *KVStore, key string, defaultValue T) (T, error) {
	value, err := Get[T](s, key)
	if err == ErrNotFound || err == ErrExpired {
		return defaultValue, nil
	}
	return value, err
}

// Delete removes a key from the store.
// Returns true if the key was found and removed, false otherwise.
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
		// Skip expired keys
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// Count returns the number of valid (non-expired) entries in the store.
func (s *KVStore) Count() int {
	keys := s.ListKeys() // This already filters out expired keys
	return len(keys)
}

// ListTypes returns the set of all concrete types stored, as their Go‐syntax names.
func (s *KVStore) ListTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[reflect.Type]struct{}{}
	out := []string{}

	for _, e := range s.data {
		// Skip expired entries
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

// KeysByType[T] returns all keys whose stored value has type T.
func KeysByType[T any](s *KVStore) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	want := reflect.TypeOf((*T)(nil)).Elem()
	keys := []string{}

	for k, e := range s.data {
		// Skip expired entries
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		if e.typ == want {
			keys = append(keys, k)
		}
	}
	return keys
}

// TypeToSchema converts a reflect.Type to a JSON-compatible map representing
// its schema structure. It handles primitives, structs, slices, maps, etc.
func TypeToSchema(t reflect.Type) interface{} {
	// Create a new instance of the type
	instance := reflect.New(t).Interface()

	// Use jsonschema library to generate the schema
	reflector := jsonschema.Reflector{
		ExpandedStruct: true,
	}
	schema := reflector.Reflect(instance)

	return schema
}

// GetTypeSchema returns a JSON Schema representation of the stored value's type.
// Returns ErrNotFound if the key doesn't exist.
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

	// Check if expired
	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		s.Delete(key)
		return nil, ErrExpired
	}

	return TypeToSchema(e.typ), nil
}

// GetAllTypeSchemas returns a map containing type schemas for all non-expired keys.
func (s *KVStore) GetAllTypeSchemas() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{})
	for k, e := range s.data {
		// Skip expired entries
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}
		result[k] = TypeToSchema(e.typ)
	}
	return result
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

// UpdateField updates a single field in a stored object using dot notation for nested fields.
// For example, to update the City field in an Address struct field:
//   - store.UpdateField("user:john", "Address.City", "New York")
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

	// Check if expired
	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		delete(s.data, key)
		return ErrExpired
	}

	// Create a new instance of the stored type and unmarshal the data
	instance := reflect.New(e.typ).Interface()
	if err := json.Unmarshal(e.blob, instance); err != nil {
		return err
	}

	// Use xreflect to set the field value
	if err := xreflect.SetEmbedField(instance, fieldPath, fieldValue); err != nil {
		return fmt.Errorf("failed to update field: %w", err)
	}

	// Marshal the object back to JSON
	newBlob, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	// Update the entry with the new blob (same type and expiration)
	s.data[key] = entry{
		typ:       e.typ,
		blob:      newBlob,
		expiresAt: e.expiresAt,
	}

	return nil
}

// UpdateFields updates multiple fields in a stored object in a single operation.
// The fields parameter is a map of field paths to their new values.
func (s *KVStore) UpdateFields(key string, fields map[string]interface{}) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}

	if len(fields) == 0 {
		return nil // Nothing to update
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		return ErrNotFound
	}

	// Check if expired
	if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
		delete(s.data, key)
		return ErrExpired
	}

	// Create a new instance of the stored type and unmarshal the data
	instance := reflect.New(e.typ).Interface()
	if err := json.Unmarshal(e.blob, instance); err != nil {
		return err
	}

	// Apply all field updates
	for fieldPath, fieldValue := range fields {
		if err := xreflect.SetEmbedField(instance, fieldPath, fieldValue); err != nil {
			return fmt.Errorf("failed to update field %s: %w", fieldPath, err)
		}
	}

	// Marshal the object back to JSON
	newBlob, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	// Update the entry with the new blob (same type and expiration)
	s.data[key] = entry{
		typ:       e.typ,
		blob:      newBlob,
		expiresAt: e.expiresAt,
	}

	return nil
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

// FindKeyCollisions identifies keys that exist in both this store and the other store.
func (s *KVStore) FindKeyCollisions(other *KVStore) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	var collisions []string
	for k, e := range s.data {
		// Skip expired entries in this store
		if e.expiresAt != nil && time.Now().After(*e.expiresAt) {
			continue
		}

		// Check if the key exists in the other store
		if otherEntry, exists := other.data[k]; exists {
			// Skip expired entries in the other store
			if otherEntry.expiresAt != nil && time.Now().After(*otherEntry.expiresAt) {
				continue
			}

			collisions = append(collisions, k)
		}
	}

	return collisions
}

// Merge combines the contents of another KVStore into this one.
// The strategy parameter determines how collisions are handled:
// - Skip: Keeps existing values when keys collide
// - Overwrite: Replaces existing values with values from the other store
// - Error: Returns an error if any keys collide
//
// Returns a list of keys that were affected by collisions and any error that occurred.
func (s *KVStore) Merge(other *KVStore, strategy MergeStrategy) ([]string, error) {
	// First, identify potential collisions
	collisions := s.FindKeyCollisions(other)

	// If strategy is Error and there are collisions, return an error
	if strategy == Error && len(collisions) > 0 {
		return collisions, fmt.Errorf("merge failed due to %d key collision(s)", len(collisions))
	}

	// Lock both stores for writing
	s.mu.Lock()
	defer s.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	// Merge the other store into this one
	for k, otherEntry := range other.data {
		// Skip expired entries
		if otherEntry.expiresAt != nil && time.Now().After(*otherEntry.expiresAt) {
			continue
		}

		// Check for collisions
		if _, exists := s.data[k]; exists {
			if strategy == Skip {
				// Skip this key if it already exists
				continue
			}
			// For Overwrite, we'll proceed with adding the key
		}

		// Copy the entry to this store
		s.data[k] = otherEntry
	}

	return collisions, nil
}

// MergeInto merges this store into the target store.
// This is the inverse operation of Merge.
func (s *KVStore) MergeInto(target *KVStore, strategy MergeStrategy) ([]string, error) {
	return target.Merge(s, strategy)
}

// ---- example usage ----

type Address struct {
	City string
	Zip  int
}

type User struct {
	Name    string
	Age     int
	Address Address
}

func main() {
	store := NewKVStore()

	alice := User{"Alice", 30, Address{"Montreal", 12345}}
	if err := store.Put("user:alice", alice); err != nil {
		panic(err)
	}

	// Store a value with TTL
	bob := User{"Bob", 25, Address{"Toronto", 54321}}
	if err := store.PutWithTTL("user:bob", bob, 5*time.Second); err != nil {
		panic(err)
	}

	// Store a more complex structure to test the schema functionality
	type Department struct {
		Name     string
		Location string
		Budget   float64
	}

	type Company struct {
		Name        string
		Departments []Department
		Metadata    map[string]string
		Active      bool
	}

	company := Company{
		Name: "Acme Inc",
		Departments: []Department{
			{Name: "Engineering", Location: "Building A", Budget: 1000000.0},
			{Name: "Sales", Location: "Building B", Budget: 500000.0},
		},
		Metadata: map[string]string{
			"founded": "1985",
			"ceo":     "John Doe",
		},
		Active: true,
	}

	store.Put("company:acme", company)

	// correct retrieval
	u, err := Get[User](store, "user:alice")
	if err != nil {
		fmt.Println("Get failed:", err)
	} else {
		fmt.Printf("got: %+v\n", u)
	}

	// wrong‑type retrieval
	type Other struct{ Foo string }
	if _, err := Get[Other](store, "user:alice"); err != nil {
		fmt.Println("expected type error:", err)
	}

	// GetOrDefault example
	defaultUser := User{"Default", 0, Address{"Unknown", 0}}
	u2, err := GetOrDefault[User](store, "non:existent", defaultUser)
	if err != nil {
		fmt.Println("GetOrDefault failed:", err)
	} else {
		fmt.Printf("GetOrDefault: %+v\n", u2)
	}

	fmt.Println("all keys:", store.ListKeys())
	fmt.Println("all types:", store.ListTypes())
	fmt.Println("keys of type User:", KeysByType[User](store))
	fmt.Println("total count:", store.Count())

	// Test JSON Schema functionality
	fmt.Println("\nJSON Schema for User type:")
	userSchema, err := store.GetTypeSchema("user:alice")
	if err != nil {
		fmt.Println("Error getting schema:", err)
	} else {
		schemaJSON, _ := json.MarshalIndent(userSchema, "", "  ")
		fmt.Println(string(schemaJSON))
	}

	fmt.Println("\nJSON Schema for Company type:")
	companySchema, _ := store.GetTypeSchema("company:acme")
	companySchemaJSON, _ := json.MarshalIndent(companySchema, "", "  ")
	fmt.Println(string(companySchemaJSON))

	// Test FindKeysBySchema - find all objects with an Address property
	fmt.Println("\nFinding keys with Address property:")
	addressPattern := map[string]interface{}{
		"properties": map[string]interface{}{
			"Address": map[string]interface{}{},
		},
	}
	matchingKeys := store.FindKeysBySchema(addressPattern)
	fmt.Println("Keys with Address property:", matchingKeys)

	// Test FindKeysBySchema - find all objects with Departments array
	fmt.Println("\nFinding keys with Departments property:")
	departmentsPattern := map[string]interface{}{
		"properties": map[string]interface{}{
			"Departments": map[string]interface{}{},
		},
	}
	deptMatchingKeys := store.FindKeysBySchema(departmentsPattern)
	fmt.Println("Keys with Departments property:", deptMatchingKeys)

	// Test partial updates using xreflect
	fmt.Println("\nTesting partial updates:")

	// Create a user with nested structure
	userToUpdate := User{"Charlie", 40, Address{"Chicago", 60601}}
	store.Put("user:charlie", userToUpdate)

	// Update a simple field
	if err := store.UpdateField("user:charlie", "Age", 41); err != nil {
		fmt.Println("Error updating age:", err)
	}

	// Update a nested field
	if err := store.UpdateField("user:charlie", "Address.City", "Boston"); err != nil {
		fmt.Println("Error updating city:", err)
	}

	// Retrieve the updated user
	updatedUser, _ := Get[User](store, "user:charlie")
	fmt.Printf("After single field updates: %+v\n", updatedUser)

	// Update multiple fields at once
	err = store.UpdateFields("user:charlie", map[string]interface{}{
		"Name":        "Charles",
		"Address.Zip": 2108,
	})
	if err != nil {
		fmt.Println("Error updating multiple fields:", err)
	}

	// Retrieve the final user
	finalUser, _ := Get[User](store, "user:charlie")
	fmt.Printf("After multi-field update: %+v\n", finalUser)

	// Test store merging and collision detection
	fmt.Println("\nTesting store merging:")

	// Create two stores with some overlapping keys
	store1 := NewKVStore()
	store2 := NewKVStore()

	// Add some data to store1
	store1.Put("key1", "Value 1 from store1")
	store1.Put("key2", "Value 2 from store1")
	store1.Put("shared", "Shared key from store1")

	// Add some data to store2
	store2.Put("key3", "Value 3 from store2")
	store2.Put("key4", "Value 4 from store2")
	store2.Put("shared", "Shared key from store2")

	// Find collisions
	collisions := store1.FindKeyCollisions(store2)
	fmt.Println("Key collisions:", collisions)

	// Merge with Skip strategy
	store1Copy := NewKVStore()
	for k, e := range store1.data {
		store1Copy.data[k] = e
	}

	affected, err := store1Copy.Merge(store2, Skip)
	if err != nil {
		fmt.Println("Merge error:", err)
	} else {
		fmt.Println("Merge with Skip - affected keys:", affected)
		fmt.Println("All keys after Skip merge:", store1Copy.ListKeys())
		v, _ := Get[string](store1Copy, "shared")
		fmt.Println("Value of 'shared' after Skip merge:", v)
	}

	// Merge with Overwrite strategy
	store1Copy = NewKVStore()
	for k, e := range store1.data {
		store1Copy.data[k] = e
	}

	affected, err = store1Copy.Merge(store2, Overwrite)
	if err != nil {
		fmt.Println("Merge error:", err)
	} else {
		fmt.Println("Merge with Overwrite - affected keys:", affected)
		fmt.Println("All keys after Overwrite merge:", store1Copy.ListKeys())
		v, _ := Get[string](store1Copy, "shared")
		fmt.Println("Value of 'shared' after Overwrite merge:", v)
	}

	// Merge with Error strategy
	store1Copy = NewKVStore()
	for k, e := range store1.data {
		store1Copy.data[k] = e
	}

	affected, err = store1Copy.Merge(store2, Error)
	if err != nil {
		fmt.Println("Merge with Error strategy:", err)
	} else {
		fmt.Println("Merge completed without error (unexpected)")
	}

	// TTL demonstration
	fmt.Println("\nBefore expiry - has bob:", KeysByType[User](store))
	fmt.Println("waiting for TTL to expire...")
	time.Sleep(6 * time.Second)
	fmt.Println("after expiry - has bob:", KeysByType[User](store))

	// Test Delete
	success := store.Delete("user:alice")
	fmt.Println("deleted alice:", success)
	fmt.Println("after delete:", store.ListKeys())

	// Test Clear
	store.Clear()
	fmt.Println("after clear - count:", store.Count())
}

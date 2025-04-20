package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetadata(t *testing.T) {
	store := NewKVStore()

	// Test adding entry with metadata
	meta := NewMetadata()
	meta.AddTag("important")
	meta.AddTag("test")
	meta.SetProperty("priority", 1)
	meta.Description = "Test value with metadata"

	err := store.PutWithMetadata("key1", "value1", meta)
	assert.NoError(t, err)

	// Test getting metadata
	retrievedMeta, err := store.GetMetadata("key1")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(retrievedMeta.Tags))
	assert.True(t, retrievedMeta.HasTag("important"))
	assert.True(t, retrievedMeta.HasTag("test"))

	// Test properties
	priority, ok := retrievedMeta.GetProperty("priority")
	assert.True(t, ok)
	assert.Equal(t, 1, priority)

	// Test description
	assert.Equal(t, "Test value with metadata", retrievedMeta.Description)

	// Test adding another entry with different tags
	meta2 := NewMetadata()
	meta2.AddTag("optional")
	meta2.AddTag("test")

	err = store.PutWithMetadata("key2", "value2", meta2)
	assert.NoError(t, err)

	// Test has tag
	hasTag, err := store.HasTag("key1", "important")
	assert.NoError(t, err)
	assert.True(t, hasTag)

	hasTag, err = store.HasTag("key2", "important")
	assert.NoError(t, err)
	assert.False(t, hasTag)

	// Test finding keys by tag
	keysWithImportant := store.FindKeysByTag("important")
	assert.Equal(t, 1, len(keysWithImportant))
	assert.Equal(t, "key1", keysWithImportant[0])

	keysWithTest := store.FindKeysByTag("test")
	assert.Equal(t, 2, len(keysWithTest))
	assert.Contains(t, keysWithTest, "key1")
	assert.Contains(t, keysWithTest, "key2")

	// Test finding keys by multiple tags
	keysWithAllTags := store.FindKeysByAllTags([]string{"important", "test"})
	assert.Equal(t, 1, len(keysWithAllTags))
	assert.Equal(t, "key1", keysWithAllTags[0])

	keysWithAnyTag := store.FindKeysByAnyTag([]string{"important", "optional"})
	assert.Equal(t, 2, len(keysWithAnyTag))

	// Test finding keys by property
	keysWithPriority := store.FindKeysByProperty("priority", 1)
	assert.Equal(t, 1, len(keysWithPriority))
	assert.Equal(t, "key1", keysWithPriority[0])

	// Test adding tag to existing key
	err = store.AddTag("key2", "important")
	assert.NoError(t, err)

	keysWithImportant = store.FindKeysByTag("important")
	assert.Equal(t, 2, len(keysWithImportant))

	// Test removing tag
	err = store.RemoveTag("key2", "important")
	assert.NoError(t, err)

	keysWithImportant = store.FindKeysByTag("important")
	assert.Equal(t, 1, len(keysWithImportant))

	// Test setting property
	err = store.SetProperty("key2", "priority", 2)
	assert.NoError(t, err)

	keysWithPriority = store.FindKeysByProperty("priority", 2)
	assert.Equal(t, 1, len(keysWithPriority))
	assert.Equal(t, "key2", keysWithPriority[0])

	// Test store merge with metadata
	otherStore := NewKVStore()
	otherMeta := NewMetadata()
	otherMeta.AddTag("shared")
	otherMeta.SetProperty("source", "other-store")

	err = otherStore.PutWithMetadata("key3", "value3", otherMeta)
	assert.NoError(t, err)

	// Add an entry with the same key but different metadata
	otherMeta2 := NewMetadata()
	otherMeta2.AddTag("important")
	otherMeta2.AddTag("shared")
	otherMeta2.SetProperty("priority", 3)

	err = otherStore.PutWithMetadata("key1", "new-value", otherMeta2)
	assert.NoError(t, err)

	// Merge stores
	collisions, err := store.Merge(otherStore, Overwrite)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(collisions))
	assert.Equal(t, "key1", collisions[0])

	// Check that metadata was merged for the collision
	mergedMeta, err := store.GetMetadata("key1")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(mergedMeta.Tags))
	assert.True(t, mergedMeta.HasTag("important"))
	assert.True(t, mergedMeta.HasTag("test"))
	assert.True(t, mergedMeta.HasTag("shared"))

	priority, ok = mergedMeta.GetProperty("priority")
	assert.True(t, ok)
	assert.Equal(t, 3, priority)

	// Check that new key was added with its metadata
	key3Meta, err := store.GetMetadata("key3")
	assert.NoError(t, err)
	assert.True(t, key3Meta.HasTag("shared"))

	source, ok := key3Meta.GetProperty("source")
	assert.True(t, ok)
	assert.Equal(t, "other-store", source)
}

func TestMetadataWithTTL(t *testing.T) {
	store := NewKVStore()

	// Create metadata
	meta := NewMetadata()
	meta.AddTag("temporary")
	meta.SetProperty("expires", true)

	// Add an entry with short TTL
	err := store.PutWithTTLAndMetadata("temp-key", "temp-value", 100*time.Millisecond, meta)
	assert.NoError(t, err)

	// Verify metadata is available immediately
	tempMeta, err := store.GetMetadata("temp-key")
	assert.NoError(t, err)
	assert.True(t, tempMeta.HasTag("temporary"))

	// Should be found in tag search
	keysWithTag := store.FindKeysByTag("temporary")
	assert.Equal(t, 1, len(keysWithTag))

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Metadata should no longer be available
	_, err = store.GetMetadata("temp-key")
	assert.Equal(t, ErrExpired, err)

	// Should no longer be found in tag search
	keysWithTag = store.FindKeysByTag("temporary")
	assert.Equal(t, 0, len(keysWithTag))
}

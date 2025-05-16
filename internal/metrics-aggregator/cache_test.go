/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package metrics_aggregator

import (
	"testing"
)

func TestNewCache(t *testing.T) {
	cache := NewCache(10)
	if cache.cacheSize != 10 {
		t.Errorf("Expected cache size to be 10, got %d", cache.cacheSize)
	}
	if len(cache.cache) != 0 {
		t.Errorf("Expected empty cache, got %d items", len(cache.cache))
	}
	if cap(cache.keysByAge) != 10 {
		t.Errorf("Expected keysByAge capacity to be 10, got %d", cap(cache.keysByAge))
	}
}

func TestCacheGetSet(t *testing.T) {
	cache := NewCache(2)

	cache.Set("key1", "value1")
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %s", value)
	}

	// Test getting a non-existent key
	_, exists = cache.Get("nonexistent")
	if exists {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestCacheEviction(t *testing.T) {
	cache := NewCache(2)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Add one more item to trigger eviction
	cache.Set("key3", "value3")

	// key1 should be evicted
	_, exists := cache.Get("key1")
	if exists {
		t.Error("Expected key1 to be evicted")
	}

	// key2 and key3 should exist
	_, exists = cache.Get("key2")
	if !exists {
		t.Error("Expected key2 to exist")
	}
	_, exists = cache.Get("key3")
	if !exists {
		t.Error("Expected key3 to exist")
	}
}

func TestCacheOverwrite(t *testing.T) {
	cache := NewCache(2)

	cache.Set("key1", "value1")
	cache.Set("key1", "new_value1")

	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected key1 to exist")
	}
	if value != "new_value1" {
		t.Errorf("Expected new_value1, got %s", value)
	}

	// Test that the cache size and keysByAge size still 1
	if len(cache.cache) != 1 {
		t.Errorf("Expected cache size to be 1, got %d", len(cache.cache))
	}
	if len(cache.keysByAge) != 1 {
		t.Errorf("Expected keysByAge length to be 2, got %d", len(cache.keysByAge))
	}
}

func TestCacheConcurrency(t *testing.T) {
	cache := NewCache(100)
	done := make(chan bool)

	// Start multiple goroutines to write to the cache
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				key := string('a'+byte(id)) + string('0'+byte(j))
				cache.Set(key, key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the cache contains all items
	count := 0
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			key := string('a'+byte(i)) + string('0'+byte(j))
			if val, exists := cache.Get(key); exists && val == key {
				count++
			}
		}
	}

	if count != 100 {
		t.Errorf("Expected 100 items in cache, found %d", count)
	}
}

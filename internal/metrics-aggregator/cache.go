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
	"sync"
)

type Cache struct {
	cache     map[string]cacheItem
	keysByAge []string
	cacheSize int
	mu        sync.RWMutex
}

type cacheItem struct {
	Value string
}

func NewCache(size int) *Cache {
	return &Cache{
		cacheSize: size,
		cache:     make(map[string]cacheItem, size),
		keysByAge: make([]string, 0, size),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if item, exists := c.cache[key]; exists {
		return item.Value, true
	}
	return "", false
}
func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.cache[key]; exists {
		c.removeKeyFromKeyByAge(key)
	}
	if len(c.cache) >= c.cacheSize {
		c.evict()
	}
	c.keysByAge = append(c.keysByAge, key)
	c.cache[key] = cacheItem{
		Value: value,
	}
}

func (c *Cache) evict() {
	oldestKey := c.keysByAge[0]
	c.keysByAge = c.keysByAge[1:]
	delete(c.cache, oldestKey)
}

func (c *Cache) removeKeyFromKeyByAge(key string) {
	for i, k := range c.keysByAge {
		if k == key {
			c.keysByAge = append(c.keysByAge[:i], c.keysByAge[i+1:]...)
			break
		}
	}
}

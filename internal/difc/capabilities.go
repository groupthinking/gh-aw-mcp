package difc

import "sync"

// Capabilities represents the global set of tags available in the system
// This is used to validate and discover available DIFC tags
type Capabilities struct {
	tags map[Tag]struct{}
	mu   sync.RWMutex
}

// NewCapabilities creates a new empty capabilities set
func NewCapabilities() *Capabilities {
	return &Capabilities{
		tags: make(map[Tag]struct{}),
	}
}

// Add adds a tag to the capabilities
func (c *Capabilities) Add(tag Tag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tags[tag] = struct{}{}
}

// AddAll adds multiple tags to the capabilities
func (c *Capabilities) AddAll(tags []Tag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, tag := range tags {
		c.tags[tag] = struct{}{}
	}
}

// Contains checks if a tag is available in the capabilities
func (c *Capabilities) Contains(tag Tag) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.tags[tag]
	return ok
}

// GetAll returns all available tags
func (c *Capabilities) GetAll() []Tag {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tags := make([]Tag, 0, len(c.tags))
	for tag := range c.tags {
		tags = append(tags, tag)
	}
	return tags
}

// Remove removes a tag from the capabilities
func (c *Capabilities) Remove(tag Tag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tags, tag)
}

// Clear removes all tags from the capabilities
func (c *Capabilities) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tags = make(map[Tag]struct{})
}

// Count returns the number of available tags
func (c *Capabilities) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tags)
}

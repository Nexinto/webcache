// dheilema 2018
// web cache

/*
Package webcache is a lightweight memory cache for go web servers that you
can wrap around time consuming requests. The content type is []byte
to allow a simple modification of the code.

If you have an long running function returning []byte which is called
with every request
 func apiHandler(w http.ResponseWriter, r *http.Request) {
   ...
   w.Write(complexBackendFunction()) // takes long time to come back
 }
you can modify this to write the backend function into a CachedPage struct
and update it only if the content is too old.
 apiCache = webcache.NewCachedPage(time.Second * 90) // cache for 90 seconds
 ...
 func apiHandler(w http.ResponseWriter, r *http.Request) {
   ...
   if !apiCache.Valid() {
     if apiCache.StartUpdate() == nil {
       apiCache.Write(complexBackendFunction())
       apiCache.EndUpdate()
     }
   }
 }
 w.Write(apiCache.Get())
You can find a longer introduction and an example server at
https://github.com/Nexinto/webcache
*/
package webcache

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// Error codes returned by this package
var (
	ErrUpdateInProgress   = errors.New("Another Update is already running")       // StartUpdate() can be called just once
	ErrWriteWithoutUpdate = errors.New("Writing to struct without StartUpdate()") // Write() is only allowed after StartUpdate()
)

// CachedPage stores the page data. There is no direct access to the fields
// and a mutex is used to protect updates.
type CachedPage struct {
	sync.RWMutex
	updating     bool
	content      []byte
	rebuild      []byte
	lastModified time.Time
	maxAge       time.Duration
	requests     uint64
	updates      uint64
}

// NewCachedPage creates a new cached page.
//
// maxAge defines how long the content will stay valid
// after an update.
func NewCachedPage(maxAge time.Duration) CachedPage {
	c := CachedPage{}
	c.maxAge = maxAge
	return c
}

// Valid reports if the cached content is valid (not aged out)
func (c *CachedPage) Valid() (v bool) {
	c.RLock()
	defer c.RUnlock()
	t1 := c.lastModified.Add(c.maxAge)
	if t1.After(time.Now()) || c.updating {
		v = true
	}
	return
}

// Clear invalidates the cache.
func (c *CachedPage) Clear() (v bool) {
	c.Lock()
	defer c.Unlock()
	c.lastModified = time.Now().Add(-c.maxAge)
	return
}

// Write implements io.Writer.
func (c *CachedPage) Write(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	if !c.updating {
		return 0, ErrWriteWithoutUpdate
	}
	n := len(p)
	c.rebuild = append(c.rebuild, p...)
	return n, nil
}

// StartUpdate marks the update transaction as "in progress".
func (c *CachedPage) StartUpdate() error {
	c.Lock()
	defer c.Unlock()
	if c.updating {
		return ErrUpdateInProgress
	}
	c.updating = true
	c.rebuild = []byte{}
	return nil
}

// EndUpdate marks the update transaction as "finished".
func (c *CachedPage) EndUpdate() {
	c.Lock()
	defer c.Unlock()
	c.content = c.rebuild
	c.lastModified = time.Now()
	c.updating = false
	c.updates++
}

// Get returns the content.
func (c *CachedPage) Get() (out []byte) {
	c.RLock()
	out = c.content
	c.RUnlock()
	c.Lock()
	c.requests++
	c.Unlock()
	return
}

// GetLastModified returns the time in rfc7232 format.
// Set the header of the response writer with
//  w.Header().Set("Last-Modified", apiCache.GetLastModified())
func (c *CachedPage) GetLastModified() (out string) {
	c.RLock()
	out = c.lastModified.Format(http.TimeFormat)
	c.RUnlock()
	return
}

// GetStatistics returns  metrics of requests handled by cache
// and number of updates.
func (c *CachedPage) GetStatistics() (requests, updates uint64) {
	c.RLock()
	defer c.RUnlock()
	requests = c.requests
	updates = c.updates
	return
}

// ClearStatistics reset the statistics counter
func (c *CachedPage) ClearStatistics() (requests, updates uint64) {
	c.Lock()
	defer c.Unlock()
	c.requests = 0
	c.updates = 0
	return
}

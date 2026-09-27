package probe

import (
    "sync"

    "github.com/wyw14/solo-037/internal/model"
)

type Collection struct {
    mu sync.Mutex
    samples map[model.RunID][]model.Evidence
    sources map[model.RunID]map[string]struct{}
    revisions map[model.RunID]model.RevisionID
}

func NewCollection() *Collection {
    return &Collection{samples: make(map[model.RunID][]model.Evidence), sources: make(map[model.RunID]map[string]struct{}), revisions: make(map[model.RunID]model.RevisionID)}
}

func (c *Collection) Add(evidence model.Evidence) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.samples[evidence.Run] = append(c.samples[evidence.Run], evidence)
    if c.sources[evidence.Run] == nil {
        c.sources[evidence.Run] = make(map[string]struct{})
    }
    c.sources[evidence.Run][evidence.SourceID] = struct{}{}
    c.revisions[evidence.Run] = evidence.Revision
}

func (c *Collection) Durable(run model.RunID) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    for _, evidence := range c.samples[run] {
        if !evidence.Durable {
            return false
        }
    }
    return len(c.samples[run]) >= 1
}

func (c *Collection) IndependentSources(run model.RunID) int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return len(c.sources[run])
}

// IndependentPhysicalSources counts distinct physical probes behind the
// run's evidence, resolving channels through the registry. Channels without
// a covering binding do not contribute.
func (c *Collection) IndependentPhysicalSources(run model.RunID, registry *Registry) int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return len(ResolvePhysical(c.samples[run], registry).Probes)
}

func (c *Collection) Samples(run model.RunID) []model.Evidence {
    c.mu.Lock()
    defer c.mu.Unlock()
    result := append([]model.Evidence{}, c.samples[run]...)
    return result
}

func (c *Collection) Revision(run model.RunID) model.RevisionID {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.revisions[run]
}

func (c *Collection) Invalidate(run model.RunID, revision model.RevisionID) {
    c.mu.Lock()
    defer c.mu.Unlock()
    retained := make([]model.Evidence, 0)
    for _, evidence := range c.samples[run] {
        if evidence.Revision == revision {
            retained = append(retained, evidence)
        }
    }
    c.samples[run] = retained
}

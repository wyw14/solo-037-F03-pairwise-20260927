package probe

import (
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

type Coordinator struct {
    Collection *Collection
    Window *CalmWindow
}

func NewCoordinator(run model.RunID, revision model.RevisionID, required time.Duration) *Coordinator {
    return &Coordinator{Collection: NewCollection(), Window: NewCalmWindow(run, revision, required)}
}

func (c *Coordinator) Record(run model.RunID, revision model.RevisionID, source string, height float64, at time.Time) model.Evidence {
    evidence := model.NewEvidence(run, revision, source, model.EvidenceWaveTime, height, at)
    c.Collection.Add(evidence)
    c.Window.Observe(Sample{Run: run, Revision: revision, Source: source, Height: height, At: at})
    return evidence
}

func (c *Coordinator) IsCalm() bool { return c.Window.Valid() }

func (c *Coordinator) Sources(run model.RunID) int { return c.Collection.IndependentSources(run) }

// PhysicalSources counts independent physical probes for the run once
// channels are resolved through the registry.
func (c *Coordinator) PhysicalSources(run model.RunID, registry *Registry) int {
    return c.Collection.IndependentPhysicalSources(run, registry)
}

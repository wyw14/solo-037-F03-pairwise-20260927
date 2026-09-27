package probe

import (
    "sort"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

// PhysicalSource groups evidence by the physical probe it came from, at one
// wiring generation. A probe that was reconnected to a new acquisition
// channel mid-run appears once per generation but still counts as a single
// independent source.
type PhysicalSource struct {
    Physical   string    `json:"physical"`
    Generation int       `json:"generation"`
    Registered bool      `json:"registered"`
    Channels   []string  `json:"channels"`
    Count      int       `json:"count"`
    First      time.Time `json:"first"`
    Last       time.Time `json:"last"`
}

// AttributeSources resolves every sample against the registry at the sample's
// own recording time, so a binding's effective window decides which physical
// probe (and generation) the evidence supports.
func AttributeSources(samples []model.Evidence, registry *Registry) []PhysicalSource {
    type key struct {
        physical   string
        generation int
    }
    grouped := make(map[key]*PhysicalSource)
    channels := make(map[key]map[string]struct{})
    order := make([]key, 0)
    for _, sample := range samples {
        binding, registered := registry.Resolve(sample.SourceID, sample.RecordedAt)
        k := key{binding.Physical, binding.Generation}
        source, ok := grouped[k]
        if !ok {
            source = &PhysicalSource{Physical: k.physical, Generation: k.generation, Registered: registered, First: sample.RecordedAt, Last: sample.RecordedAt}
            grouped[k] = source
            channels[k] = make(map[string]struct{})
            order = append(order, k)
        }
        source.Count++
        if sample.RecordedAt.Before(source.First) {
            source.First = sample.RecordedAt
        }
        if sample.RecordedAt.After(source.Last) {
            source.Last = sample.RecordedAt
        }
        channels[k][sample.SourceID] = struct{}{}
    }
    result := make([]PhysicalSource, 0, len(grouped))
    for _, k := range order {
        source := grouped[k]
        for channel := range channels[k] {
            source.Channels = append(source.Channels, channel)
        }
        sort.Strings(source.Channels)
        result = append(result, *source)
    }
    sort.Slice(result, func(i, j int) bool {
        if result[i].Physical != result[j].Physical {
            return result[i].Physical < result[j].Physical
        }
        return result[i].Generation < result[j].Generation
    })
    return result
}

// IndependentPhysical counts distinct physical probes, no matter how many
// channels or generations each one contributed.
func IndependentPhysical(sources []PhysicalSource) int {
    seen := make(map[string]struct{}, len(sources))
    for _, source := range sources {
        seen[source.Physical] = struct{}{}
    }
    return len(seen)
}

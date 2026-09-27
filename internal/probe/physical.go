package probe

import (
    "math"
    "sort"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

// PhysicalGroup is the evidence backing one generation of one physical
// probe. Independence between wave sources is judged on probes, never on
// channels: two channels bound to the same probe are one physical source.
type PhysicalGroup struct {
    Probe      string        `json:"probe"`
    Generation int           `json:"generation"`
    Channels   []string      `json:"channels"`
    Count      int           `json:"count"`
    First      time.Time     `json:"first"`
    Last       time.Time     `json:"last"`
    Span       time.Duration `json:"span"`
    RMS        float64       `json:"rms"`
    Durable    bool          `json:"durable"`
}

// PhysicalAnalysis is the quality conclusion of a set of samples expressed
// in physical terms: which probes produced evidence, through which
// generation and channels, and which channels could not be resolved to any
// registered probe inside its valid window.
type PhysicalAnalysis struct {
    Groups     []PhysicalGroup `json:"groups"`
    Probes     []string        `json:"probes"`
    Unresolved []string        `json:"unresolved,omitempty"`
}

// ResolvePhysical attributes every sample to a physical probe generation.
// A nil registry keeps the legacy identity mapping where every channel is
// treated as its own physical source. With a registry, samples whose
// channel has no binding covering their recording time do not support
// independence; their channels are reported in Unresolved.
func ResolvePhysical(samples []model.Evidence, registry *Registry) PhysicalAnalysis {
    type key struct {
        probe      string
        generation int
    }
    groups := make(map[key][]model.Evidence)
    order := make([]key, 0)
    unresolved := make(map[string]struct{})
    for _, sample := range samples {
        probeID := sample.SourceID
        generation := 0
        if registry != nil {
            binding, ok := registry.Resolve(sample.SourceID, sample.RecordedAt)
            if !ok {
                unresolved[sample.SourceID] = struct{}{}
                continue
            }
            probeID = binding.Probe
            generation = binding.Generation
        }
        groupKey := key{probe: probeID, generation: generation}
        if _, seen := groups[groupKey]; !seen {
            order = append(order, groupKey)
        }
        groups[groupKey] = append(groups[groupKey], sample)
    }
    result := PhysicalAnalysis{}
    probeSet := make(map[string]struct{})
    for _, groupKey := range order {
        members := groups[groupKey]
        sort.SliceStable(members, func(i, j int) bool { return members[i].RecordedAt.Before(members[j].RecordedAt) })
        group := PhysicalGroup{
            Probe:      groupKey.probe,
            Generation: groupKey.generation,
            Count:      len(members),
            First:      members[0].RecordedAt,
            Last:       members[len(members)-1].RecordedAt,
            Durable:    true,
        }
        group.Span = group.Last.Sub(group.First)
        squares := 0.0
        channels := make(map[string]struct{})
        for _, member := range members {
            squares += member.Value * member.Value
            channels[member.SourceID] = struct{}{}
            if !member.Durable {
                group.Durable = false
            }
        }
        group.RMS = math.Sqrt(squares / float64(group.Count))
        for channel := range channels {
            group.Channels = append(group.Channels, channel)
        }
        sort.Strings(group.Channels)
        result.Groups = append(result.Groups, group)
        if _, seen := probeSet[group.Probe]; !seen {
            probeSet[group.Probe] = struct{}{}
            result.Probes = append(result.Probes, group.Probe)
        }
    }
    sort.Strings(result.Probes)
    for channel := range unresolved {
        result.Unresolved = append(result.Unresolved, channel)
    }
    sort.Strings(result.Unresolved)
    return result
}

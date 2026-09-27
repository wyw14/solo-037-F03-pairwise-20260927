package probe

import (
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func TestRegistryResolvesBindingWithinEffectiveWindow(t *testing.T) {
    registry := NewRegistry()
    from := time.Unix(1700000000, 0)
    to := from.Add(time.Hour)
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: to}); err != nil {
        t.Fatal(err)
    }
    binding, registered := registry.Resolve("eth-0", from.Add(time.Minute))
    if !registered || binding.Physical != "probe-1" || binding.Generation != 1 {
        t.Fatalf("expected registered probe-1 gen1, got %+v registered=%v", binding, registered)
    }
    if _, registered := registry.Resolve("eth-0", to.Add(time.Minute)); registered {
        t.Fatal("binding resolved outside its effective window")
    }
    if _, registered := registry.Resolve("eth-0", from.Add(-time.Minute)); registered {
        t.Fatal("binding resolved before its effective window")
    }
}

func TestRegistryRejectsInvalidAndOverlappingBindings(t *testing.T) {
    registry := NewRegistry()
    from := time.Unix(1700000000, 0)
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "", Generation: 1}); err == nil {
        t.Fatal("empty physical accepted")
    }
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 0}); err == nil {
        t.Fatal("zero generation accepted")
    }
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: from}); err == nil {
        t.Fatal("empty window accepted")
    }
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: from.Add(time.Hour)}); err != nil {
        t.Fatal(err)
    }
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-9", Generation: 2, EffectiveFrom: from.Add(30 * time.Minute)}); err == nil {
        t.Fatal("overlapping window for the same channel accepted")
    }
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 2, EffectiveFrom: from.Add(time.Hour)}); err != nil {
        t.Fatalf("adjacent window rejected: %v", err)
    }
}

func TestReconnectedProbeCountsAsOnePhysicalSource(t *testing.T) {
    registry := NewRegistry()
    run := model.NewRunID()
    revision := model.NewRevisionID()
    from := time.Unix(1700000000, 0)
    swap := from.Add(time.Hour)
    // The same physical probe is first wired to eth-0, then re-connected to
    // usb-3 at a new generation.
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: swap}); err != nil {
        t.Fatal(err)
    }
    if _, err := registry.Register(ChannelBinding{Channel: "usb-3", Physical: "probe-1", Generation: 2, EffectiveFrom: swap}); err != nil {
        t.Fatal(err)
    }
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "eth-0", model.EvidenceWaveTime, 0.01, from.Add(time.Minute)),
        model.NewEvidence(run, revision, "usb-3", model.EvidenceWaveTime, 0.02, swap.Add(time.Minute)),
    }
    sources := AttributeSources(samples, registry)
    if len(sources) != 2 {
        t.Fatalf("expected one group per generation, got %+v", sources)
    }
    if IndependentPhysical(sources) != 1 {
        t.Fatalf("reconnected probe counted as more than one physical source: %+v", sources)
    }
    for _, source := range sources {
        if source.Physical != "probe-1" || !source.Registered || len(source.Channels) != 1 {
            t.Fatalf("unexpected attribution: %+v", source)
        }
    }
    if sources[0].Generation != 1 || sources[1].Generation != 2 {
        t.Fatalf("generations not preserved: %+v", sources)
    }
}

func TestUnregisteredChannelFallsBackToSelfDeclaredIdentity(t *testing.T) {
    run := model.NewRunID()
    revision := model.NewRevisionID()
    at := time.Unix(1700000000, 0)
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "probe-a", model.EvidenceWaveTime, 0.01, at),
        model.NewEvidence(run, revision, "probe-b", model.EvidenceWaveTime, 0.02, at.Add(time.Second)),
    }
    sources := AttributeSources(samples, nil)
    if len(sources) != 2 || IndependentPhysical(sources) != 2 {
        t.Fatalf("legacy channels should stay independent, got %+v", sources)
    }
    for _, source := range sources {
        if source.Registered || source.Generation != 0 || source.Physical != source.Channels[0] {
            t.Fatalf("fallback identity should be self-declared gen0: %+v", source)
        }
    }
}

func TestSampleOutsideBindingWindowDoesNotInheritPhysicalIdentity(t *testing.T) {
    registry := NewRegistry()
    run := model.NewRunID()
    revision := model.NewRevisionID()
    from := time.Unix(1700000000, 0)
    if _, err := registry.Register(ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: from.Add(time.Hour)}); err != nil {
        t.Fatal(err)
    }
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "eth-0", model.EvidenceWaveTime, 0.01, from.Add(time.Minute)),
        model.NewEvidence(run, revision, "eth-0", model.EvidenceWaveTime, 0.02, from.Add(2*time.Hour)),
    }
    sources := AttributeSources(samples, registry)
    if len(sources) != 2 {
        t.Fatalf("expected windowed and out-of-window samples to split, got %+v", sources)
    }
    if IndependentPhysical(sources) != 2 {
        t.Fatalf("out-of-window evidence must not support the registered probe: %+v", sources)
    }
}

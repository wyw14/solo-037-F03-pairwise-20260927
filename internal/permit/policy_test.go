package permit

import (
    "strings"
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
    "github.com/wyw14/solo-037/internal/probe"
)

func waveSamples(run model.RunID, revision model.RevisionID, source string, base time.Time, count int) []model.Evidence {
    samples := make([]model.Evidence, 0, count)
    for index := 0; index < count; index++ {
        samples = append(samples, model.NewEvidence(run, revision, source, model.EvidenceWaveTime, 0.01, base.Add(time.Duration(index)*time.Second)).WithDurability(true))
    }
    return samples
}

func requirementOf(decision Decision, name string) Requirement {
    for _, requirement := range decision.Requirements {
        if requirement.Name == name {
            return requirement
        }
    }
    return Requirement{Name: name}
}

func TestPolicyDoesNotCountReconnectedProbeAsTwoSources(t *testing.T) {
    registry := probe.NewRegistry()
    run := model.NewRunID()
    revision := model.NewRevisionID()
    from := time.Unix(1700000000, 0)
    swap := from.Add(time.Hour)
    for _, binding := range []probe.ChannelBinding{
        {Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: swap},
        {Channel: "usb-3", Physical: "probe-1", Generation: 2, EffectiveFrom: swap},
    } {
        if _, err := registry.Register(binding); err != nil {
            t.Fatal(err)
        }
    }
    samples := append(waveSamples(run, revision, "eth-0", from.Add(time.Minute), 2), waveSamples(run, revision, "usb-3", swap.Add(time.Minute), 2)...)
    decision := DefaultPolicy().EvaluateWithRegistry(run, samples, registry, true, true, swap.Add(2*time.Minute))
    sources := requirementOf(decision, "independent_sources")
    if sources.Satisfied {
        t.Fatalf("one physical probe satisfied the two-source requirement: %+v", decision)
    }
    if decision.Allowed {
        t.Fatal("permit granted on a single physical probe")
    }
    if len(decision.Supporting) != 2 || decision.Supporting[0].Physical != "probe-1" {
        t.Fatalf("supporting evidence should attribute both channels to probe-1: %+v", decision.Supporting)
    }
    if !strings.Contains(sources.Detail, "probe-1(gen1)<=eth-0") || !strings.Contains(sources.Detail, "probe-1(gen2)<=usb-3") {
        t.Fatalf("detail should explain which channels back the probe: %q", sources.Detail)
    }
}

func TestPolicyAcceptsTwoRegisteredPhysicalProbes(t *testing.T) {
    registry := probe.NewRegistry()
    run := model.NewRunID()
    revision := model.NewRevisionID()
    from := time.Unix(1700000000, 0)
    for _, binding := range []probe.ChannelBinding{
        {Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from},
        {Channel: "eth-1", Physical: "probe-2", Generation: 1, EffectiveFrom: from},
    } {
        if _, err := registry.Register(binding); err != nil {
            t.Fatal(err)
        }
    }
    samples := append(waveSamples(run, revision, "eth-0", from.Add(time.Minute), 2), waveSamples(run, revision, "eth-1", from.Add(time.Minute), 2)...)
    decision := DefaultPolicy().EvaluateWithRegistry(run, samples, registry, true, true, from.Add(2*time.Minute))
    if !requirementOf(decision, "independent_sources").Satisfied {
        t.Fatalf("two registered probes should satisfy independence: %+v", decision)
    }
    if !decision.Allowed {
        t.Fatalf("permit denied despite complete evidence: %+v", decision)
    }
    if probe.IndependentPhysical(decision.Supporting) != 2 {
        t.Fatalf("supporting evidence should name two physical probes: %+v", decision.Supporting)
    }
}

func TestPolicyWithoutRegistryKeepsLegacyChannelSemantics(t *testing.T) {
    run := model.NewRunID()
    revision := model.NewRevisionID()
    from := time.Unix(1700000000, 0)
    samples := append(waveSamples(run, revision, "probe-a", from, 2), waveSamples(run, revision, "probe-b", from, 2)...)
    decision := DefaultPolicy().Evaluate(run, samples, true, true, from.Add(time.Minute))
    if !decision.Allowed {
        t.Fatalf("legacy unregistered channels should still count independently: %+v", decision)
    }
    if !strings.Contains(requirementOf(decision, "independent_sources").Detail, "unregistered") {
        t.Fatalf("self-declared sources should be marked unregistered: %+v", decision.Requirements)
    }
}

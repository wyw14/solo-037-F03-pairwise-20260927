package permit

import (
    "strings"
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
    "github.com/wyw14/solo-037/internal/probe"
)

func waveSamples(run model.RunID, revision model.RevisionID, sources ...string) []model.Evidence {
    base := time.Unix(1700000000, 0).UTC()
    samples := make([]model.Evidence, 0, len(sources))
    for index, source := range sources {
        sample := model.NewEvidence(run, revision, source, model.EvidenceWaveTime, 0.01, base.Add(time.Duration(index)*100*time.Millisecond))
        samples = append(samples, sample.WithDurability(true))
    }
    return samples
}

func TestPolicyCountsPhysicalProbesNotChannels(t *testing.T) {
    registry := probe.NewRegistry()
    base := time.Unix(1700000000, 0).UTC()
    // One physical probe moved to a new interface mid-run: two channels,
    // one source. The permit must not treat them as independent.
    if _, err := registry.Register("left-usb", "probe-1", "operator", base.Add(-time.Minute), time.Time{}, base); err != nil {
        t.Fatal(err)
    }
    if _, err := registry.Register("left-ethernet", "probe-1", "operator", base.Add(-time.Minute), time.Time{}, base); err != nil {
        t.Fatal(err)
    }
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := waveSamples(run, revision, "left-usb", "left-ethernet")
    decision := DefaultPolicy().EvaluateWithRegistry(run, samples, registry, true, true, base)
    requirements := RequirementMap(decision)
    if requirements["independent_sources"] {
        t.Fatalf("interface swap counted as two independent sources: %+v", decision.Requirements)
    }
    if decision.Allowed {
        t.Fatal("permit granted on a single physical probe")
    }
    if len(decision.Support) == 0 || decision.Support[0].Probe != "probe-1" {
        t.Fatalf("decision does not name its supporting probe: %+v", decision.Support)
    }
}

func TestPolicyExplainsWhichChannelsSupportTheConclusion(t *testing.T) {
    registry := probe.NewRegistry()
    base := time.Unix(1700000000, 0).UTC()
    if _, err := registry.Register("left-usb", "probe-1", "operator", base.Add(-time.Minute), time.Time{}, base); err != nil {
        t.Fatal(err)
    }
    if _, err := registry.Register("right-usb", "probe-2", "operator", base.Add(-time.Minute), time.Time{}, base); err != nil {
        t.Fatal(err)
    }
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := waveSamples(run, revision, "left-usb", "right-usb", "spare")
    decision := DefaultPolicy().EvaluateWithRegistry(run, samples, registry, true, true, base)
    if !decision.Allowed {
        t.Fatalf("two registered probes should satisfy the permit: %+v", decision.Requirements)
    }
    var detail string
    for _, requirement := range decision.Requirements {
        if requirement.Name == "independent_sources" {
            detail = requirement.Detail
        }
    }
    if !strings.Contains(detail, "probe-1") || !strings.Contains(detail, "left-usb") || !strings.Contains(detail, "probe-2") {
        t.Fatalf("detail does not explain supporting channels: %q", detail)
    }
    if !strings.Contains(detail, "unresolved:spare") || len(decision.Unresolved) != 1 || decision.Unresolved[0] != "spare" {
        t.Fatalf("unregistered channel not explained: %q %+v", detail, decision.Unresolved)
    }
    if len(decision.Support) != 2 {
        t.Fatalf("support should list one group per probe generation: %+v", decision.Support)
    }
}

func TestPolicyWithoutRegistryKeepsLegacyChannelCounting(t *testing.T) {
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := waveSamples(run, revision, "left-usb", "left-ethernet")
    decision := DefaultPolicy().Evaluate(run, samples, true, true, time.Unix(1700000000, 0))
    if !RequirementMap(decision)["independent_sources"] {
        t.Fatalf("legacy evaluation must keep channel identity: %+v", decision.Requirements)
    }
}

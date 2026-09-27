package probe

import (
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func TestRegistryResolvesInterfaceSwapToOnePhysicalSource(t *testing.T) {
    registry := NewRegistry()
    base := time.Unix(1700000000, 0).UTC()
    if _, err := registry.Register("left-usb", "probe-1", "operator", base, base.Add(time.Hour), base); err != nil {
        t.Fatal(err)
    }
    swapped, err := registry.Register("left-ethernet", "probe-1", "operator", base.Add(time.Hour), time.Time{}, base.Add(time.Hour))
    if err != nil {
        t.Fatal(err)
    }
    if swapped.Generation != 2 {
        t.Fatalf("interface swap should start generation 2, got %d", swapped.Generation)
    }
    if _, err := registry.Register("right-usb", "probe-2", "operator", base, time.Time{}, base); err != nil {
        t.Fatal(err)
    }
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "left-usb", model.EvidenceWaveTime, 0.01, base.Add(time.Minute)),
        model.NewEvidence(run, revision, "left-ethernet", model.EvidenceWaveTime, 0.02, base.Add(2*time.Hour)),
        model.NewEvidence(run, revision, "right-usb", model.EvidenceWaveTime, 0.01, base.Add(time.Minute)),
    }
    physical := ResolvePhysical(samples, registry)
    if len(physical.Probes) != 2 {
        t.Fatalf("interface swap counted as extra source: %+v", physical.Probes)
    }
    if len(physical.Groups) != 3 {
        t.Fatalf("generations must stay separate quality groups: %+v", physical.Groups)
    }
    for _, group := range physical.Groups {
        if group.Probe == "probe-1" && group.Generation == 1 && (len(group.Channels) != 1 || group.Channels[0] != "left-usb") {
            t.Fatalf("generation 1 lost its channel: %+v", group)
        }
        if group.Probe == "probe-1" && group.Generation == 2 && (len(group.Channels) != 1 || group.Channels[0] != "left-ethernet") {
            t.Fatalf("generation 2 lost its channel: %+v", group)
        }
    }
}

func TestRegistryRejectsInvalidAndCountsOnlyCoveredEvidence(t *testing.T) {
    registry := NewRegistry()
    base := time.Unix(1700000000, 0).UTC()
    if _, err := registry.Register("ch", "probe-1", "", base, time.Time{}, base); err == nil {
        t.Fatal("registrar must be required")
    }
    if _, err := registry.Register("ch", "probe-1", "operator", base, base.Add(-time.Minute), base); err == nil {
        t.Fatal("window ending before it starts accepted")
    }
    if _, err := registry.Register("known", "probe-1", "operator", base, base.Add(time.Hour), base); err != nil {
        t.Fatal(err)
    }
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "known", model.EvidenceWaveTime, 0.01, base.Add(time.Minute)),
        model.NewEvidence(run, revision, "known", model.EvidenceWaveTime, 0.01, base.Add(2*time.Hour)),
        model.NewEvidence(run, revision, "mystery", model.EvidenceWaveTime, 0.01, base.Add(time.Minute)),
    }
    physical := ResolvePhysical(samples, registry)
    if len(physical.Probes) != 1 || physical.Probes[0] != "probe-1" {
        t.Fatalf("out-of-window and unregistered channels leaked into sources: %+v", physical)
    }
    if len(physical.Unresolved) != 2 || physical.Unresolved[0] != "known" || physical.Unresolved[1] != "mystery" {
        t.Fatalf("channels with uncovered samples not reported: %+v", physical.Unresolved)
    }
    if physical.Groups[0].Count != 1 {
        t.Fatalf("sample outside the valid window joined the group: %+v", physical.Groups[0])
    }
}

func TestResolvePhysicalWithoutRegistryKeepsLegacyIdentity(t *testing.T) {
    run := model.NewRunID()
    revision := model.NewRevisionID()
    samples := []model.Evidence{
        model.NewEvidence(run, revision, "a", model.EvidenceWaveTime, 0.01, time.Unix(1700000000, 0)),
        model.NewEvidence(run, revision, "b", model.EvidenceWaveTime, 0.01, time.Unix(1700000000, 0)),
    }
    physical := ResolvePhysical(samples, nil)
    if len(physical.Probes) != 2 || len(physical.Unresolved) != 0 {
        t.Fatalf("nil registry must keep channel identity: %+v", physical)
    }
}

func TestRestoreContinuesGenerationSequence(t *testing.T) {
    registry := NewRegistry()
    base := time.Unix(1700000000, 0).UTC()
    first, err := registry.Register("ch", "probe-1", "operator", base, time.Time{}, base)
    if err != nil {
        t.Fatal(err)
    }
    restored := NewRegistry()
    restored.Restore(first)
    second, err := restored.Register("ch2", "probe-1", "operator", base, time.Time{}, base)
    if err != nil {
        t.Fatal(err)
    }
    if second.Generation != 2 {
        t.Fatalf("restored registry restarted generations: %+v", second)
    }
}

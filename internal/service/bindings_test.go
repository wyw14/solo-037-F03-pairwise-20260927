package service

import (
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func TestBindingStorePersistsAcrossReopen(t *testing.T) {
    directory := t.TempDir()
    store, err := OpenBindingStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    from := time.Now().Add(-time.Hour).UTC()
    binding, err := store.Register(BindingInput{Channel: "left-usb", Probe: "probe-1", ValidFrom: from, RegisteredBy: "operator"})
    if err != nil {
        t.Fatal(err)
    }
    if binding.Generation != 1 {
        t.Fatalf("first registration must be generation 1: %+v", binding)
    }
    reloaded, err := OpenBindingStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    if len(reloaded.List()) != 1 {
        t.Fatalf("bindings not persisted: %+v", reloaded.List())
    }
    resolved, ok := reloaded.Registry().Resolve("left-usb", time.Now().UTC())
    if !ok || resolved.Probe != "probe-1" || resolved.Generation != 1 {
        t.Fatalf("reloaded registry cannot resolve: %+v %v", resolved, ok)
    }
    followUp, err := reloaded.Register(BindingInput{Channel: "left-ethernet", Probe: "probe-1", ValidFrom: from, RegisteredBy: "operator"})
    if err != nil {
        t.Fatal(err)
    }
    if followUp.Generation != 2 {
        t.Fatalf("generation sequence broken after reload: %+v", followUp)
    }
}

func TestAnalysisResolvesInterfaceSwapThroughRegistry(t *testing.T) {
    runtime, err := OpenRuntime(t.TempDir())
    if err != nil {
        t.Fatal(err)
    }
    run, err := runtime.CreateMeasuredRun("swap", "hull X", "section-a", "carriage-1")
    if err != nil {
        t.Fatal(err)
    }
    from := time.Now().Add(-time.Hour).UTC()
    for _, channel := range []string{"left-usb", "left-ethernet"} {
        if _, err := runtime.RegisterProbeBinding(BindingInput{Channel: channel, Probe: "probe-1", ValidFrom: from, RegisteredBy: "operator"}); err != nil {
            t.Fatal(err)
        }
    }
    at := time.Now().Add(-time.Minute).UTC()
    for index, channel := range []string{"left-usb", "left-ethernet"} {
        input := MeasurementInput{Kind: model.EvidenceWaveTime, SourceID: channel, Value: 0.01, RecordedAt: at.Add(time.Duration(index) * time.Second)}
        if _, err := runtime.RecordMeasurement(run.Operation.ID, input); err != nil {
            t.Fatal(err)
        }
    }
    analysis, err := runtime.AnalyzeMeasuredRun(run.Operation.ID)
    if err != nil {
        t.Fatal(err)
    }
    if len(analysis.Physical.Probes) != 1 || analysis.Physical.Probes[0] != "probe-1" {
        t.Fatalf("interface swap inflated physical sources: %+v", analysis.Physical)
    }
    if len(analysis.Physical.Groups) != 2 {
        t.Fatalf("generations must remain distinct quality groups: %+v", analysis.Physical.Groups)
    }
    if len(analysis.Physical.Unresolved) != 0 {
        t.Fatalf("registered channels reported unresolved: %+v", analysis.Physical.Unresolved)
    }
}

func TestAnalysisReportsUnregisteredChannels(t *testing.T) {
    runtime, err := OpenRuntime(t.TempDir())
    if err != nil {
        t.Fatal(err)
    }
    run, err := runtime.CreateMeasuredRun("unregistered", "hull X", "section-a", "carriage-1")
    if err != nil {
        t.Fatal(err)
    }
    input := MeasurementInput{Kind: model.EvidenceWaveTime, SourceID: "mystery", Value: 0.01, RecordedAt: time.Now().Add(-time.Minute).UTC()}
    if _, err := runtime.RecordMeasurement(run.Operation.ID, input); err != nil {
        t.Fatal(err)
    }
    analysis, err := runtime.AnalyzeMeasuredRun(run.Operation.ID)
    if err != nil {
        t.Fatal(err)
    }
    if len(analysis.Physical.Probes) != 0 || len(analysis.Physical.Unresolved) != 1 || analysis.Physical.Unresolved[0] != "mystery" {
        t.Fatalf("unregistered channel must not support the conclusion: %+v", analysis.Physical)
    }
}

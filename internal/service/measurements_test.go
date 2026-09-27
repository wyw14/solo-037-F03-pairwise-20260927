package service

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/solo-037/internal/model"
	"github.com/wyw14/solo-037/internal/probe"
)

func TestOperatorMeasurementsPersistAndAnalyzeSuppliedValues(t *testing.T) {
	directory := t.TempDir()
	a, err := OpenMeasurementArchive(directory)
	if err != nil {
		t.Fatal(err)
	}
	run, err := a.Create("test A", "hull X", "section-a", "carriage-1")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Minute).UTC()
	for _, input := range []MeasurementInput{
		{Kind: model.EvidenceWaveTime, SourceID: "left", Value: 0.1, RecordedAt: at},
		{Kind: model.EvidenceWaveTime, SourceID: "right", Value: 0.3, RecordedAt: at.Add(time.Second)},
		{Kind: model.EvidenceForce, SourceID: "load-cell", Value: 12, RecordedAt: at},
	} {
		if _, err := a.Record(run.Operation.ID, input); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := OpenMeasurementArchive(directory)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := loaded.Analyze(run.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Wave.Count != 2 || math.Abs(analysis.Wave.Mean-0.2) > 1e-9 || analysis.ForceCount != 1 || analysis.ForceMean != 12 {
		t.Fatalf("analysis did not reflect supplied values after reload: %+v", analysis)
	}
	if len(loaded.List()) != 1 || len(loaded.List()[0].Evidence) != 3 || !loaded.List()[0].Evidence[0].Durable {
		t.Fatalf("persisted evidence missing: %+v", loaded.List())
	}
}

func TestOperatorMeasurementRejectsInvalidAndUnknownRun(t *testing.T) {
	a, err := OpenMeasurementArchive(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run, err := a.Create("test B", "hull X", "section-a", "carriage-1")
	if err != nil {
		t.Fatal(err)
	}
	input := MeasurementInput{Kind: model.EvidenceWaveTime, SourceID: "left", Value: 1, RecordedAt: time.Now().UTC()}
	if _, err := a.Record("missing", input); err == nil {
		t.Fatal("unknown run accepted")
	}
	input.Value = math.NaN()
	if _, err := a.Record(run.Operation.ID, input); err == nil {
		t.Fatal("NaN accepted")
	}
	if len(a.List()[0].Evidence) != 0 {
		t.Fatal("rejected values changed archive")
	}
}

func TestProbeBindingsPersistAndShapeSourceJudgement(t *testing.T) {
	directory := t.TempDir()
	a, err := OpenMeasurementArchive(directory)
	if err != nil {
		t.Fatal(err)
	}
	run, err := a.Create("test C", "hull Y", "section-b", "carriage-2")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Now().Add(-time.Hour).UTC()
	swap := from.Add(30 * time.Minute)
	// One physical probe, reconnected to a new channel mid-run.
	if _, err := a.RegisterProbeBinding(probe.ChannelBinding{Channel: "eth-0", Physical: "probe-1", Generation: 1, EffectiveFrom: from, EffectiveTo: swap}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RegisterProbeBinding(probe.ChannelBinding{Channel: "usb-3", Physical: "probe-1", Generation: 2, EffectiveFrom: swap}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []MeasurementInput{
		{Kind: model.EvidenceWaveTime, SourceID: "eth-0", Value: 0.01, RecordedAt: from.Add(time.Minute)},
		{Kind: model.EvidenceWaveTime, SourceID: "usb-3", Value: 0.02, RecordedAt: swap.Add(time.Minute)},
	} {
		if _, err := a.Record(run.Operation.ID, input); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := OpenMeasurementArchive(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ProbeBindings()) != 2 {
		t.Fatalf("bindings not persisted: %+v", loaded.ProbeBindings())
	}
	analysis, err := loaded.Analyze(run.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.PhysicalSources) != 2 || probe.IndependentPhysical(analysis.PhysicalSources) != 1 {
		t.Fatalf("reconnected probe should collapse to one physical source: %+v", analysis.PhysicalSources)
	}
	decision, err := loaded.EvaluateMeasuredRun(run.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, requirement := range decision.Requirements {
		if requirement.Name == "independent_sources" {
			if requirement.Satisfied {
				t.Fatalf("single physical probe satisfied independence: %+v", decision)
			}
			if !strings.Contains(requirement.Detail, "probe-1(gen1)<=eth-0") || !strings.Contains(requirement.Detail, "probe-1(gen2)<=usb-3") {
				t.Fatalf("detail does not explain the supporting channels: %q", requirement.Detail)
			}
		}
	}
	if len(decision.Supporting) != 2 || decision.Supporting[0].Physical != "probe-1" {
		t.Fatalf("decision should name the supporting physical probe: %+v", decision.Supporting)
	}
	if _, err := loaded.RegisterProbeBinding(probe.ChannelBinding{Channel: "eth-0", Physical: "probe-9", Generation: 3, EffectiveFrom: from}); err == nil {
		t.Fatal("overlapping binding accepted after reload")
	}
}

package permit

import (
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/wyw14/solo-037/internal/model"
    "github.com/wyw14/solo-037/internal/probe"
)

type Requirement struct {
    Name string `json:"name"`
    Satisfied bool `json:"satisfied"`
    Detail string `json:"detail"`
}

type Decision struct {
    Run model.RunID `json:"run"`
    Allowed bool `json:"allowed"`
    Requirements []Requirement `json:"requirements"`
    Support []probe.PhysicalGroup `json:"support"`
    Unresolved []string `json:"unresolved,omitempty"`
    EvaluatedAt time.Time `json:"evaluated_at"`
}

type Policy struct {
    MinimumSources int
    MaximumGap time.Duration
    MaximumWaveRMS float64
}

func DefaultPolicy() Policy {
    return Policy{MinimumSources: 2, MaximumGap: 1500 * time.Millisecond, MaximumWaveRMS: 0.03}
}

func (p Policy) Evaluate(run model.RunID, samples []model.Evidence, calm bool, brake bool, at time.Time) Decision {
    return p.EvaluateWithRegistry(run, samples, nil, calm, brake, at)
}

// EvaluateWithRegistry judges independence on physical probes resolved
// through the registry, so a probe that changed acquisition interface is
// not counted twice. The decision explains its basis: Support lists the
// probe generations whose evidence carried the conclusion, Unresolved
// lists channels that no registered binding covered.
func (p Policy) EvaluateWithRegistry(run model.RunID, samples []model.Evidence, registry *probe.Registry, calm bool, brake bool, at time.Time) Decision {
    matching := probe.FilterRun(samples, run)
    analysis := probe.Analyze(matching)
    physical := probe.ResolvePhysical(matching, registry)
    requirements := []Requirement{
        {Name: "independent_sources", Satisfied: len(physical.Probes) >= p.MinimumSources, Detail: physicalDetail(physical)},
        {Name: "continuous_window", Satisfied: probe.Continuous(matching, p.MaximumGap), Detail: analysis.Span.String()},
        {Name: "durable_wave_evidence", Satisfied: probe.Durable(matching), Detail: durabilityDetail(matching)},
        {Name: "calm_water", Satisfied: calm && analysis.RMS <= p.MaximumWaveRMS, Detail: waveDetail(analysis.RMS)},
        {Name: "carriage_brake", Satisfied: brake, Detail: brakeDetail(brake)},
    }
    allowed := true
    for _, requirement := range requirements {
        allowed = allowed && requirement.Satisfied
    }
    return Decision{Run: run, Allowed: allowed, Requirements: requirements, Support: physical.Groups, Unresolved: physical.Unresolved, EvaluatedAt: at.UTC()}
}

func physicalDetail(physical probe.PhysicalAnalysis) string {
    labels := make(map[string][]string, len(physical.Groups))
    probes := make([]string, 0, len(physical.Groups))
    for _, group := range physical.Groups {
        if _, seen := labels[group.Probe]; !seen {
            probes = append(probes, group.Probe)
        }
        labels[group.Probe] = append(labels[group.Probe], physicalGroupLabel(group))
    }
    sort.Strings(probes)
    parts := make([]string, 0, len(probes))
    for _, probeID := range probes {
        parts = append(parts, probeID+"="+strings.Join(labels[probeID], "+"))
    }
    result := strings.Join(parts, ";")
    if len(physical.Unresolved) > 0 {
        result += "; unresolved:" + strings.Join(physical.Unresolved, ",")
    }
    return result
}

func physicalGroupLabel(group probe.PhysicalGroup) string {
    return "g" + strconv.Itoa(group.Generation) + "<-" + strings.Join(group.Channels, ",")
}

func durabilityDetail(samples []model.Evidence) string {
    durable := 0
    for _, sample := range samples {
        if sample.Durable {
            durable++
        }
    }
    return formatCount(durable, len(samples))
}

func waveDetail(value float64) string {
    if value <= 0.01 {
        return "calm"
    }
    if value <= 0.03 {
        return "settling"
    }
    return "active"
}

func brakeDetail(value bool) string {
    if value {
        return "current command stopped"
    }
    return "current command proof missing"
}

func formatCount(current int, total int) string {
    digits := func(value int) string {
        if value == 0 {
            return "0"
        }
        result := ""
        for value > 0 {
            result = string(rune('0'+value%10)) + result
            value /= 10
        }
        return result
    }
    return digits(current) + "/" + digits(total)
}

func Missing(decision Decision) []string {
    result := make([]string, 0)
    for _, requirement := range decision.Requirements {
        if !requirement.Satisfied {
            result = append(result, requirement.Name)
        }
    }
    sort.Strings(result)
    return result
}

func RequirementMap(decision Decision) map[string]bool {
    result := make(map[string]bool, len(decision.Requirements))
    for _, requirement := range decision.Requirements {
        result[requirement.Name] = requirement.Satisfied
    }
    return result
}

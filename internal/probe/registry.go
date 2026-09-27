package probe

import (
    "errors"
    "fmt"
    "sort"
    "strings"
    "sync"
    "time"
)

// ChannelBinding records which physical probe an acquisition channel belongs
// to, at which wiring generation, and the window in which that assignment is
// valid. Personnel register a binding whenever a probe is (re)connected, so a
// probe that moves to a new interface keeps its physical identity instead of
// appearing as a brand new independent source.
type ChannelBinding struct {
    Channel       string    `json:"channel"`
    Physical      string    `json:"physical"`
    Generation    int       `json:"generation"`
    EffectiveFrom time.Time `json:"effective_from"`
    EffectiveTo   time.Time `json:"effective_to,omitempty"`
}

func (b ChannelBinding) Covers(at time.Time) bool {
    if at.Before(b.EffectiveFrom) {
        return false
    }
    return b.EffectiveTo.IsZero() || at.Before(b.EffectiveTo)
}

func windowEnd(at time.Time) time.Time {
    if at.IsZero() {
        return time.Unix(1<<62, 0)
    }
    return at
}

func (b ChannelBinding) Overlaps(other ChannelBinding) bool {
    if b.Channel != other.Channel {
        return false
    }
    return b.EffectiveFrom.Before(windowEnd(other.EffectiveTo)) && other.EffectiveFrom.Before(windowEnd(b.EffectiveTo))
}

// Registry holds the operator-declared channel bindings. It is pure state:
// persistence lives with the caller.
type Registry struct {
    mu       sync.Mutex
    bindings []ChannelBinding
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(binding ChannelBinding) (ChannelBinding, error) {
    binding.Channel = strings.TrimSpace(binding.Channel)
    binding.Physical = strings.TrimSpace(binding.Physical)
    if binding.Channel == "" || binding.Physical == "" {
        return ChannelBinding{}, errors.New("channel and physical probe are required")
    }
    if binding.Generation < 1 {
        return ChannelBinding{}, errors.New("generation must be a positive integer")
    }
    if !binding.EffectiveTo.IsZero() && !binding.EffectiveTo.After(binding.EffectiveFrom) {
        return ChannelBinding{}, errors.New("effective window must end after it starts")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    for _, existing := range r.bindings {
        if existing.Overlaps(binding) {
            return ChannelBinding{}, fmt.Errorf("channel %s already bound to probe %s generation %d in an overlapping window", binding.Channel, existing.Physical, existing.Generation)
        }
    }
    r.bindings = append(r.bindings, binding)
    return binding, nil
}

// Resolve returns the binding governing a channel at a moment. Channels with
// no covering registration fall back to a self-declared identity (generation
// 0, registered=false) so legacy evidence keeps its old meaning, but the
// conclusion can tell declared from registered sources.
func (r *Registry) Resolve(channel string, at time.Time) (ChannelBinding, bool) {
    if r == nil {
        return ChannelBinding{Channel: channel, Physical: channel}, false
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    best := -1
    for index, binding := range r.bindings {
        if binding.Channel == channel && binding.Covers(at) && (best < 0 || binding.Generation > r.bindings[best].Generation) {
            best = index
        }
    }
    if best < 0 {
        return ChannelBinding{Channel: channel, Physical: channel}, false
    }
    return r.bindings[best], true
}

func (r *Registry) Bindings() []ChannelBinding {
    if r == nil {
        return nil
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    result := append([]ChannelBinding{}, r.bindings...)
    sort.Slice(result, func(i, j int) bool {
        if result[i].Channel != result[j].Channel {
            return result[i].Channel < result[j].Channel
        }
        return result[i].Generation < result[j].Generation
    })
    return result
}

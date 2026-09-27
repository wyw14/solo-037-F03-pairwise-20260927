package probe

import (
    "errors"
    "sort"
    "strings"
    "sync"
    "time"
)

// ChannelBinding records that an acquisition channel belonged to a physical
// probe during a validity window. Re-registering a probe (for example after
// an interface swap) starts a new generation so historical evidence keeps
// the resolution that was valid when it was recorded.
type ChannelBinding struct {
    Channel      string    `json:"channel"`
    Probe        string    `json:"probe"`
    Generation   int       `json:"generation"`
    ValidFrom    time.Time `json:"valid_from"`
    ValidTo      time.Time `json:"valid_to"`
    RegisteredBy string    `json:"registered_by"`
    RegisteredAt time.Time `json:"registered_at"`
}

// Covers reports whether at falls inside [ValidFrom, ValidTo). A zero
// ValidTo means the binding is open-ended.
func (b ChannelBinding) Covers(at time.Time) bool {
    if at.Before(b.ValidFrom) {
        return false
    }
    return b.ValidTo.IsZero() || at.Before(b.ValidTo)
}

// Registry holds the operator-declared probe/channel relations. It is the
// authority that decides which physical probe produced a piece of evidence.
type Registry struct {
    mu          sync.Mutex
    bindings    []ChannelBinding
    generations map[string]int
}

func NewRegistry() *Registry {
    return &Registry{generations: make(map[string]int)}
}

// Register declares that channel belongs to probe during [from, to). The
// generation is per probe and increments on every registration for that
// probe, so evidence recorded before and after an interface swap is never
// merged into a single quality group.
func (r *Registry) Register(channel, probe, registeredBy string, from, to, at time.Time) (ChannelBinding, error) {
    if strings.TrimSpace(channel) == "" || strings.TrimSpace(probe) == "" || strings.TrimSpace(registeredBy) == "" {
        return ChannelBinding{}, errors.New("channel, probe and registrar are required")
    }
    if from.IsZero() || (!to.IsZero() && !to.After(from)) {
        return ChannelBinding{}, errors.New("valid window must start and, when set, end after it starts")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.generations[probe]++
    binding := ChannelBinding{
        Channel:      channel,
        Probe:        probe,
        Generation:   r.generations[probe],
        ValidFrom:    from.UTC(),
        ValidTo:      to.UTC(),
        RegisteredBy: registeredBy,
        RegisteredAt: at.UTC(),
    }
    r.bindings = append(r.bindings, binding)
    return binding, nil
}

// Restore appends a previously persisted binding verbatim and lifts the
// probe's generation counter so later registrations continue the sequence.
func (r *Registry) Restore(binding ChannelBinding) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.bindings = append(r.bindings, binding)
    if binding.Generation > r.generations[binding.Probe] {
        r.generations[binding.Probe] = binding.Generation
    }
}

// Resolve returns the binding that covers channel at the given time. When
// several bindings cover the instant the highest generation wins, so the
// most recent registration of a probe speaks for it.
func (r *Registry) Resolve(channel string, at time.Time) (ChannelBinding, bool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    best := ChannelBinding{}
    found := false
    for _, binding := range r.bindings {
        if binding.Channel != channel || !binding.Covers(at) {
            continue
        }
        if !found || binding.Generation > best.Generation {
            best = binding
            found = true
        }
    }
    return best, found
}

func (r *Registry) Bindings() []ChannelBinding {
    r.mu.Lock()
    defer r.mu.Unlock()
    result := append([]ChannelBinding{}, r.bindings...)
    sort.SliceStable(result, func(i, j int) bool { return result[i].RegisteredAt.Before(result[j].RegisteredAt) })
    return result
}

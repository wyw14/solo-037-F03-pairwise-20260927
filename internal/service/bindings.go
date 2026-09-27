package service

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/wyw14/solo-037/internal/probe"
)

// BindingInput is what an operator supplies to declare that an acquisition
// channel belongs to a physical probe for a validity window.
type BindingInput struct {
    Channel      string    `json:"channel"`
    Probe        string    `json:"probe"`
    ValidFrom    time.Time `json:"valid_from"`
    ValidTo      time.Time `json:"valid_to"`
    RegisteredBy string    `json:"registered_by"`
}

// BindingStore keeps the probe/channel registry durable so source
// resolution survives restarts and stays auditable.
type BindingStore struct {
    mu       sync.Mutex
    path     string
    registry *probe.Registry
    bindings []probe.ChannelBinding
}

func OpenBindingStore(directory string) (*BindingStore, error) {
    store := &BindingStore{registry: probe.NewRegistry()}
    if directory == "" {
        return store, nil
    }
    if err := os.MkdirAll(directory, 0o750); err != nil {
        return nil, err
    }
    store.path = filepath.Join(directory, "probe-bindings.json")
    data, err := os.ReadFile(store.path)
    if errors.Is(err, os.ErrNotExist) {
        return store, nil
    }
    if err != nil {
        return nil, err
    }
    var bindings []probe.ChannelBinding
    if err := json.Unmarshal(data, &bindings); err != nil {
        return nil, fmt.Errorf("read binding store: %w", err)
    }
    for _, binding := range bindings {
        store.registry.Restore(binding)
    }
    store.bindings = bindings
    return store, nil
}

func (s *BindingStore) commit() error {
    if s.path == "" {
        return nil
    }
    data, err := json.MarshalIndent(s.bindings, "", "  ")
    if err != nil {
        return err
    }
    file, err := os.CreateTemp(filepath.Dir(s.path), "bindings-*.tmp")
    if err != nil {
        return err
    }
    name := file.Name()
    defer os.Remove(name)
    if _, err = file.Write(data); err == nil {
        err = file.Sync()
    }
    closeErr := file.Close()
    if err != nil {
        return err
    }
    if closeErr != nil {
        return closeErr
    }
    return os.Rename(name, s.path)
}

// Register declares a new probe/channel relation. Every registration is a
// new generation of the probe, so an interface swap never rewrites how
// historical evidence resolves.
func (s *BindingStore) Register(input BindingInput) (probe.ChannelBinding, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    binding, err := s.registry.Register(input.Channel, input.Probe, input.RegisteredBy, input.ValidFrom, input.ValidTo, time.Now().UTC())
    if err != nil {
        return probe.ChannelBinding{}, err
    }
    s.bindings = append(s.bindings, binding)
    if err := s.commit(); err != nil {
        return probe.ChannelBinding{}, err
    }
    return binding, nil
}

func (s *BindingStore) List() []probe.ChannelBinding {
    s.mu.Lock()
    defer s.mu.Unlock()
    return append([]probe.ChannelBinding{}, s.bindings...)
}

// Registry exposes the live resolution view for quality computations.
func (s *BindingStore) Registry() *probe.Registry { return s.registry }

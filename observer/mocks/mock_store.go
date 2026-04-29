package mocks

import observer "github.com/gulducat/autoscaler-holodeck/observer"

// MockStore is a configurable mock implementation of observer.EventStore.
// Set the Func fields in tests to control behavior.
type MockStore struct {
AddFunc        func(input observer.EventInput) observer.Event
QueryFunc      func(run, since int64, kind string) (int64, []observer.Event)
CurrentRunFunc func() int64
}

func (m *MockStore) Add(input observer.EventInput) observer.Event {
return m.AddFunc(input)
}

func (m *MockStore) Query(run, since int64, kind string) (int64, []observer.Event) {
return m.QueryFunc(run, since, kind)
}

func (m *MockStore) CurrentRun() int64 {
return m.CurrentRunFunc()
}

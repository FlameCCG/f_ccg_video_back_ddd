package bootstrap

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type runtimeComponentSpy struct {
	name     string
	events   *[]string
	startErr error
	stopErr  error
}

func (s *runtimeComponentSpy) Start(context.Context) error {
	*s.events = append(*s.events, "start:"+s.name)

	return s.startErr
}

func (s *runtimeComponentSpy) Stop(context.Context) error {
	*s.events = append(*s.events, "stop:"+s.name)

	return s.stopErr
}

func TestRuntimeComponentsPreserveStartAndStopOrder(t *testing.T) {
	t.Parallel()

	var events []string

	component := func(name string) runtimeComponent {
		return &runtimeComponentSpy{name: name, events: &events}
	}

	runtime := &RuntimeComponents{
		river:     component("river"),
		video:     component("video"),
		transcode: component("transcode"),
		cron:      component("cron"),
		chatHub:   component("chat"),
		danmuHub:  component("danmu"),
	}

	runtime.Start(context.Background())
	runtime.Stop(context.Background())

	want := []string{
		"start:river", "start:video", "start:transcode", "start:cron", "start:chat", "start:danmu",
		"stop:transcode", "stop:video", "stop:river", "stop:cron", "stop:chat", "stop:danmu",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRuntimeComponentsContinueAfterComponentError(t *testing.T) {
	t.Parallel()

	var events []string

	runtime := &RuntimeComponents{
		river: &runtimeComponentSpy{
			name:     "river",
			events:   &events,
			startErr: errors.New("start failed"),
			stopErr:  errors.New("stop failed"),
		},
		video: &runtimeComponentSpy{name: "video", events: &events},
	}

	runtime.Start(context.Background())
	runtime.Stop(context.Background())

	want := []string{"start:river", "start:video", "stop:video", "stop:river"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

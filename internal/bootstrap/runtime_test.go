package bootstrap

import (
	"context"
	"reflect"
	"testing"
)

type runtimeComponentSpy struct {
	name   string
	events *[]string
}

func (s *runtimeComponentSpy) Start(context.Context) {
	*s.events = append(*s.events, "start:"+s.name)
}

func (s *runtimeComponentSpy) Stop(context.Context) {
	*s.events = append(*s.events, "stop:"+s.name)
}

func TestRuntimeComponentsPreserveStartAndStopOrder(t *testing.T) {
	t.Parallel()

	var events []string
	component := func(name string) runtimeComponent {
		return &runtimeComponentSpy{name: name, events: &events}
	}

	runtime := &RuntimeComponents{
		chatIndexes: component("chat_indexes"),
		river:       component("river"),
		video:       component("video"),
		transcode:   component("transcode"),
		cron:        component("cron"),
		chatHub:     component("chat"),
		danmuHub:    component("danmu"),
	}

	runtime.Start(context.Background())
	runtime.Stop(context.Background())

	want := []string{
		"start:chat_indexes", "start:river", "start:video", "start:transcode", "start:cron",
		"stop:transcode", "stop:video", "stop:river", "stop:cron", "stop:chat", "stop:danmu",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestRuntimeComponentsSkipMissingComponents(t *testing.T) {
	t.Parallel()

	var events []string
	runtime := &RuntimeComponents{
		river: &runtimeComponentSpy{name: "river", events: &events},
		video: &runtimeComponentSpy{name: "video", events: &events},
	}

	runtime.Start(context.Background())
	runtime.Stop(context.Background())

	want := []string{"start:river", "start:video", "stop:video", "stop:river"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

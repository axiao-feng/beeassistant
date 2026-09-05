package agentkit

import (
	"strings"
	"testing"

	domainevent "fkteams/internal/domain/event"
	domainmessage "fkteams/internal/domain/message"
)

func TestToolIdentityAttachMapsUnknownResultIDToPendingCall(t *testing.T) {
	tracker := newToolIdentityTracker()
	index := 0
	call := domainmessage.ToolCall{
		Index: &index,
		Function: domainmessage.FunctionCall{
			Name:      "echo",
			Arguments: `{"text":"hello"}`,
		},
	}
	ref := tracker.ensure("message-1", 0, MemberScope{}, &call)
	tracker.rememberResult(call.Function.Name, call.ID, MemberScope{})

	event := &domainevent.Event{
		Type:       domainevent.TypeToolCallCompleted,
		ToolCallID: "provider-real-id",
		ToolName:   "echo",
	}
	tracker.attach(event, MemberScope{})

	if !strings.HasPrefix(call.ID, "fk_tool_") {
		t.Fatalf("generated id = %q, want fk_tool_ prefix", call.ID)
	}
	if event.ToolCallID != call.ID {
		t.Fatalf("event tool id = %q, want normalized id %q", event.ToolCallID, call.ID)
	}
	if event.ToolCallRef != ref {
		t.Fatalf("event tool ref = %q, want %q", event.ToolCallRef, ref)
	}
}

func TestToolIdentityResultQueuesAreScopedByMember(t *testing.T) {
	tracker := newToolIdentityTracker()
	firstScope := MemberScope{CallID: "member-1"}
	secondScope := MemberScope{CallID: "member-2"}
	first := domainmessage.ToolCall{Function: domainmessage.FunctionCall{Name: "search"}}
	second := domainmessage.ToolCall{Function: domainmessage.FunctionCall{Name: "search"}}
	firstRef := tracker.ensure("message-1", 0, firstScope, &first)
	secondRef := tracker.ensure("message-2", 0, secondScope, &second)
	tracker.rememberResult(first.Function.Name, first.ID, firstScope)
	tracker.rememberResult(second.Function.Name, second.ID, secondScope)

	secondEvent := &domainevent.Event{
		Type:     domainevent.TypeToolCallCompleted,
		ToolName: "search",
	}
	tracker.attach(secondEvent, secondScope)
	if secondEvent.ToolCallID != second.ID || secondEvent.ToolCallRef != secondRef {
		t.Fatalf("second scoped event = %#v, want id %q ref %q", secondEvent, second.ID, secondRef)
	}

	firstEvent := &domainevent.Event{
		Type:     domainevent.TypeToolCallCompleted,
		ToolName: "search",
	}
	tracker.attach(firstEvent, firstScope)
	if firstEvent.ToolCallID != first.ID || firstEvent.ToolCallRef != firstRef {
		t.Fatalf("first scoped event = %#v, want id %q ref %q", firstEvent, first.ID, firstRef)
	}
}

package agentkit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"fkteams/internal/app/tools/ask"
	domainevent "fkteams/internal/domain/event"
	domainmessage "fkteams/internal/domain/message"
	runtimeport "fkteams/internal/ports/runtime"
	checkpointmemory "fkteams/internal/runtime/checkpoint"
	"fkteams/internal/testmodel"
)

func TestAgentToolMemberEventsKeepScopeForReasoningAndTools(t *testing.T) {
	ctx := context.Background()
	memberTool, err := runtimeport.InferTool("member_echo", "member echo", func(_ context.Context, req *memberEchoRequest) (*memberEchoResponse, error) {
		return &memberEchoResponse{Text: "tool:" + req.Text}, nil
	})
	if err != nil {
		t.Fatalf("create member tool: %v", err)
	}

	memberModel := testmodel.New().
		EnqueueStream(
			domainmessage.Message{Role: domainmessage.RoleAssistant, ReasoningContent: "member-thinking"},
			domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{
				ID:    "member-tool-call",
				Index: intPtr(0),
				Type:  "function",
				Function: domainmessage.FunctionCall{
					Name:      "member_echo",
					Arguments: `{"text":"hello"}`,
				},
			}}},
		).
		EnqueueStream(testmodel.AssistantMessage("member-done"))
	memberAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "member",
		Description:   "member",
		Model:         memberModel,
		Tools:         []runtimeport.Tool{memberTool},
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create member agent: %v", err)
	}

	agentTools, err := NewAgentTools(ctx, []runtimeport.Agent{memberAgent}, runtimeport.AgentToolConfig{
		ToolName: func(string, int) string { return "ask_fkagent_member" },
	})
	if err != nil {
		t.Fatalf("create agent tools: %v", err)
	}

	parentModel := testmodel.New().
		EnqueueStream(domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{
			ID:    "parent-member-call",
			Index: intPtr(0),
			Type:  "function",
			Function: domainmessage.FunctionCall{
				Name:      "ask_fkagent_member",
				Arguments: `{"request":"do member task"}`,
			},
		}}}).
		EnqueueStream(testmodel.AssistantMessage("parent-done"))
	parentAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "parent",
		Description:   "parent",
		Model:         parentModel,
		Tools:         agentTools,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create parent agent: %v", err)
	}

	got := runAgentForTest(t, ctx, parentAgent, true)

	parentStartIdx := requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.Type == domainevent.TypeToolCallStarted &&
			event.ToolCallID == "parent-member-call" &&
			event.ToolName == "ask_fkagent_member" &&
			event.ToolCallRef != ""
	}, "parent member tool start")
	memberReasoningIdx := requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-member-call" &&
			event.ParentToolCallID == "parent-member-call" &&
			event.MemberToolName == "ask_fkagent_member" &&
			event.MemberName == "member" &&
			event.DeltaKind == domainevent.DeltaReasoning &&
			strings.Contains(event.Content, "member-thinking")
	}, "member-scoped reasoning")
	memberToolStartIdx := requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-member-call" &&
			event.Type == domainevent.TypeToolCallStarted &&
			event.ToolName == "member_echo" &&
			event.ToolCallRef != "" &&
			event.ToolCallIndex != nil &&
			*event.ToolCallIndex == 0
	}, "member-scoped tool start")
	memberToolResultIdx := requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-member-call" &&
			(event.Type == domainevent.TypeToolCallResult || event.Type == domainevent.TypeToolCallCompleted) &&
			event.ToolName == "member_echo" &&
			event.ToolCallRef != ""
	}, "member-scoped tool result")

	requireBefore(t, got, parentStartIdx, memberReasoningIdx, "parent member tool start", "member reasoning")
	requireBefore(t, got, memberReasoningIdx, memberToolStartIdx, "member reasoning", "member tool start")
	requireBefore(t, got, memberToolStartIdx, memberToolResultIdx, "member tool start", "member tool result")
}

func TestMemberAskInterruptResumesInsideMemberAgent(t *testing.T) {
	ctx := runtimeport.WithInterruptRuntime(context.Background(), NewInterruptRuntime())
	askTools, err := ask.GetTools()
	if err != nil {
		t.Fatalf("create ask tools: %v", err)
	}

	memberModel := testmodel.New().
		EnqueueStream(domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{
			ID:    "member-ask-call",
			Index: intPtr(0),
			Type:  "function",
			Function: domainmessage.FunctionCall{
				Name:      "ask_questions",
				Arguments: `{"question":"Need input?","options":["A","B"]}`,
			},
		}}}).
		EnqueueStream(testmodel.AssistantMessage("member resumed with answer"))
	memberAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "member",
		Description:   "member",
		Model:         memberModel,
		Tools:         askTools,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create member agent: %v", err)
	}

	agentTools, err := NewAgentTools(ctx, []runtimeport.Agent{memberAgent}, runtimeport.AgentToolConfig{
		ToolName: func(string, int) string { return "ask_fkagent_member" },
	})
	if err != nil {
		t.Fatalf("create agent tools: %v", err)
	}

	parentModel := testmodel.New().
		EnqueueStream(domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{
			ID:    "parent-member-call",
			Index: intPtr(0),
			Type:  "function",
			Function: domainmessage.FunctionCall{
				Name:      "ask_fkagent_member",
				Arguments: `{"request":"ask the user and continue"}`,
			},
		}}}).
		EnqueueStream(testmodel.AssistantMessage("parent done"))
	parentAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "parent",
		Description:   "parent",
		Model:         parentModel,
		Tools:         agentTools,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create parent agent: %v", err)
	}

	runner, err := NewRunnerFromConfig(ctx, runtimeport.RunnerConfig{
		Agent:           parentAgent,
		EnableStreaming: true,
		CheckpointStore: checkpointmemory.NewMemoryStore(),
	})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	var got []domainevent.Event
	var seenMemberInterrupt bool
	_, err = runner.Run(ctx, domainmessage.TurnInput{
		Message: domainmessage.Message{Role: domainmessage.RoleUser, Content: "start"},
	}, runtimeport.RunOptions{
		RunID: "member-ask-resume-test",
		Sink: func(event domainevent.Event) error {
			got = append(got, event)
			return nil
		},
		InterruptHandler: func(_ context.Context, interrupts []runtimeport.Interrupt) (runtimeport.InterruptDecisions, error) {
			result := make(runtimeport.InterruptDecisions, len(interrupts))
			for _, ic := range interrupts {
				if ic.MemberCallID == "parent-member-call" && ic.MemberToolName == "ask_fkagent_member" {
					seenMemberInterrupt = true
				}
				if ic.IsRootCause {
					result[ic.ID] = &ask.AskResponse{Selected: []string{"A"}}
				}
			}
			return result, nil
		},
	})
	if err != nil {
		t.Fatalf("run with member ask interrupt: %v; events=%#v", err, got)
	}
	if !seenMemberInterrupt {
		t.Fatalf("member ask interrupt was not marked with parent member call")
	}

	requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-member-call" &&
			event.Type == domainevent.TypeAssistantCompleted &&
			event.Content == "member resumed with answer"
	}, "member resumed output")
	requireEventIndex(t, got, func(event domainevent.Event) bool {
		return event.Type == domainevent.TypeAssistantCompleted &&
			event.Role == domainmessage.RoleAssistant &&
			event.Content == "parent done"
	}, "parent final output")

	memberCalls := memberModel.StreamCalls()
	if len(memberCalls) < 2 {
		t.Fatalf("member model stream calls = %d, want at least 2", len(memberCalls))
	}
	if !messagesContain(memberCalls[1].Input, "A") {
		t.Fatalf("member resume input does not contain ask answer: %#v", memberCalls[1].Input)
	}
}

func TestMemberRuntimeAskDoesNotBlockParallelMember(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	askTools, err := ask.GetTools()
	if err != nil {
		t.Fatalf("create ask tools: %v", err)
	}

	askerModel := testmodel.New().
		EnqueueStream(domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{{
			ID:    "asker-ask-call",
			Index: intPtr(0),
			Type:  "function",
			Function: domainmessage.FunctionCall{
				Name:      "ask_questions",
				Arguments: `{"question":"Need input?","options":["yes","no"]}`,
			},
		}}}).
		EnqueueStream(testmodel.AssistantMessage("asker done"))
	askerAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "asker",
		Description:   "asker",
		Model:         askerModel,
		Tools:         askTools,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create asker agent: %v", err)
	}

	workerModel := testmodel.New().EnqueueStream(testmodel.AssistantMessage("worker done"))
	workerAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "worker",
		Description:   "worker",
		Model:         workerModel,
		MaxIterations: 2,
	})
	if err != nil {
		t.Fatalf("create worker agent: %v", err)
	}

	agentTools, err := NewAgentTools(ctx, []runtimeport.Agent{askerAgent, workerAgent}, runtimeport.AgentToolConfig{
		ToolName: func(name string, _ int) string { return "ask_fkagent_" + name },
	})
	if err != nil {
		t.Fatalf("create agent tools: %v", err)
	}

	parentModel := testmodel.New().
		EnqueueStream(domainmessage.Message{Role: domainmessage.RoleAssistant, ToolCalls: []domainmessage.ToolCall{
			{
				ID:    "parent-asker-call",
				Index: intPtr(0),
				Type:  "function",
				Function: domainmessage.FunctionCall{
					Name:      "ask_fkagent_asker",
					Arguments: `{"request":"ask the user"}`,
				},
			},
			{
				ID:    "parent-worker-call",
				Index: intPtr(1),
				Type:  "function",
				Function: domainmessage.FunctionCall{
					Name:      "ask_fkagent_worker",
					Arguments: `{"request":"finish independently"}`,
				},
			},
		}}).
		EnqueueStream(testmodel.AssistantMessage("parent done"))
	parentAgent, err := NewChatModelAgent(ctx, &runtimeport.ChatAgentConfig{
		Name:          "parent",
		Description:   "parent",
		Model:         parentModel,
		Tools:         agentTools,
		MaxIterations: 4,
	})
	if err != nil {
		t.Fatalf("create parent agent: %v", err)
	}

	runner, err := NewRunnerFromConfig(ctx, runtimeport.RunnerConfig{
		Agent:           parentAgent,
		EnableStreaming: true,
		CheckpointStore: checkpointmemory.NewMemoryStore(),
	})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	askReqCh := make(chan ask.RuntimeRequest, 1)
	askRespCh := make(chan *ask.AskResponse, 1)
	ctx = ask.WithRuntimeHandler(ctx, func(ctx context.Context, req ask.RuntimeRequest) (*ask.AskResponse, error) {
		askReqCh <- req
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp := <-askRespCh:
			return resp, nil
		}
	})

	eventCh := make(chan domainevent.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		_, runErr := runner.Run(ctx, domainmessage.TurnInput{
			Message: domainmessage.Message{Role: domainmessage.RoleUser, Content: "start"},
		}, runtimeport.RunOptions{
			RunID: "member-runtime-ask-parallel-test",
			Sink: func(event domainevent.Event) error {
				select {
				case eventCh <- event:
				case <-ctx.Done():
				}
				return nil
			},
			InterruptHandler: func(context.Context, []runtimeport.Interrupt) (runtimeport.InterruptDecisions, error) {
				return nil, fmt.Errorf("member runtime ask reached parent interrupt handler")
			},
		})
		errCh <- runErr
	}()

	req := waitAskRuntimeRequest(t, ctx, askReqCh)
	if req.Metadata.MemberCallID != "parent-asker-call" {
		t.Fatalf("ask member call ID = %q, want parent-asker-call", req.Metadata.MemberCallID)
	}

	waitEvent(t, ctx, eventCh, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-worker-call" &&
			event.Type == domainevent.TypeAssistantCompleted &&
			event.Content == "worker done"
	}, "worker completion before ask answer")

	askRespCh <- &ask.AskResponse{AskID: req.ID, Selected: []string{"yes"}}

	waitEvent(t, ctx, eventCh, func(event domainevent.Event) bool {
		return event.MemberCallID == "parent-asker-call" &&
			event.Type == domainevent.TypeAssistantCompleted &&
			event.Content == "asker done"
	}, "asker completion after answer")
	waitEvent(t, ctx, eventCh, func(event domainevent.Event) bool {
		return event.Type == domainevent.TypeAssistantCompleted &&
			event.Role == domainmessage.RoleAssistant &&
			event.Content == "parent done"
	}, "parent completion")

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runner error = %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("runner did not finish: %v", ctx.Err())
	}
}

func waitAskRuntimeRequest(t *testing.T, ctx context.Context, ch <-chan ask.RuntimeRequest) ask.RuntimeRequest {
	t.Helper()
	select {
	case req := <-ch:
		return req
	case <-ctx.Done():
		t.Fatalf("timed out waiting for ask runtime request: %v", ctx.Err())
		return ask.RuntimeRequest{}
	}
}

func waitEvent(t *testing.T, ctx context.Context, ch <-chan domainevent.Event, match func(domainevent.Event) bool, label string) domainevent.Event {
	t.Helper()
	for {
		select {
		case event := <-ch:
			if match(event) {
				return event
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s: %v", label, ctx.Err())
			return domainevent.Event{}
		}
	}
}

type memberEchoRequest struct {
	Text string `json:"text"`
}

type memberEchoResponse struct {
	Text string `json:"text"`
}

func messagesContain(messages []domainmessage.Message, text string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, text) {
			return true
		}
	}
	return false
}

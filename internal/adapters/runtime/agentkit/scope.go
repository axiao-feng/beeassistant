package agentkit

import domainevent "fkteams/internal/domain/event"

// MemberScope 标识一次成员委派的稳定关联。
type MemberScope struct {
	CallID   string
	ToolName string
	Name     string
}

func (s MemberScope) apply(event *domainevent.Event, c *converter) {
	if event == nil || s.CallID == "" {
		return
	}
	event.MemberCallID = s.CallID
	event.MemberToolName = s.ToolName
	event.MemberName = s.Name
	event.AgentName = s.Name
	event.ParentToolCallID = s.CallID
	event.ParentToolName = s.ToolName
	if event.MemberOrder == nil && s.CallID != "" {
		if order, ok := c.identities.orderForID(s.CallID); ok {
			event.MemberOrder = intPtr(order)
		}
	}
}

func intPtr(v int) *int {
	return &v
}

package agentkit

import (
	domainmessage "fkteams/internal/domain/message"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// AdaptMessages 将领域消息转换为 SDK 消息，供适配器中间件复用。
func AdaptMessages(messages []domainmessage.Message) []adk.Message {
	result := make([]adk.Message, 0, len(messages))
	for _, msg := range messages {
		m := &schema.Message{
			Role:             adaptRoleForRunner(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			ToolCallID:       msg.ToolCallID,
			ToolName:         msg.ToolName,
			Name:             msg.Name,
		}
		if len(msg.ContentParts) > 0 {
			if msg.Role == domainmessage.RoleAssistant {
				m.AssistantGenMultiContent = adaptOutputPartsForRunner(msg.ContentParts)
			} else {
				m.UserInputMultiContent = adaptPartsForRunner(msg.ContentParts)
			}
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = adaptToolCallsForRunner(msg.ToolCalls)
		}
		result = append(result, m)
	}
	return result
}

func adaptMessageFromRunner(msg *schema.Message) domainmessage.Message {
	if msg == nil {
		return domainmessage.Message{}
	}
	parts := adaptPartsFromRunner(msg.UserInputMultiContent)
	if len(msg.AssistantGenMultiContent) > 0 {
		parts = append(parts, adaptOutputPartsFromRunner(msg.AssistantGenMultiContent)...)
	}
	return domainmessage.Message{
		Role:             adaptRoleFromRunner(msg.Role),
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		ToolCalls:        adaptToolCallsFromRunner(msg.ToolCalls),
		ToolCallID:       msg.ToolCallID,
		ToolName:         msg.ToolName,
		ContentParts:     parts,
		Name:             msg.Name,
	}
}

func adaptRoleForRunner(role domainmessage.Role) schema.RoleType {
	switch role {
	case domainmessage.RoleSystem:
		return schema.System
	case domainmessage.RoleUser:
		return schema.User
	case domainmessage.RoleAssistant:
		return schema.Assistant
	case domainmessage.RoleTool:
		return schema.Tool
	default:
		return schema.User
	}
}

func adaptRoleFromRunner(role schema.RoleType) domainmessage.Role {
	switch role {
	case schema.System:
		return domainmessage.RoleSystem
	case schema.User:
		return domainmessage.RoleUser
	case schema.Assistant:
		return domainmessage.RoleAssistant
	case schema.Tool:
		return domainmessage.RoleTool
	default:
		return domainmessage.Role(role)
	}
}

func adaptToolCallsForRunner(toolCalls []domainmessage.ToolCall) []schema.ToolCall {
	result := make([]schema.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		result = append(result, schema.ToolCall{
			ID:    tc.ID,
			Index: tc.Index,
			Type:  tc.Type,
			Function: schema.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return result
}

func adaptToolCallsFromRunner(toolCalls []schema.ToolCall) []domainmessage.ToolCall {
	result := make([]domainmessage.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		result = append(result, adaptToolCallFromRunner(tc))
	}
	return result
}

func adaptToolCallFromRunner(tc schema.ToolCall) domainmessage.ToolCall {
	return domainmessage.ToolCall{
		ID:    tc.ID,
		Index: tc.Index,
		Type:  tc.Type,
		Function: domainmessage.FunctionCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
	}
}

func adaptPartsForRunner(parts []domainmessage.ContentPart) []schema.MessageInputPart {
	result := make([]schema.MessageInputPart, 0, len(parts))
	for _, part := range parts {
		p := schema.MessageInputPart{Text: part.Text}
		switch part.Type {
		case domainmessage.ContentPartText:
			p.Type = schema.ChatMessagePartTypeText
		case domainmessage.ContentPartImageURL:
			p.Type = schema.ChatMessagePartTypeImageURL
			p.Image = &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        stringPtr(part.URL),
					Base64Data: stringPtr(part.Base64Data),
					MIMEType:   part.MIMEType,
				},
				Detail: schema.ImageURLDetail(part.Detail),
			}
		case domainmessage.ContentPartAudioURL:
			p.Type = schema.ChatMessagePartTypeAudioURL
			p.Audio = &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{URL: stringPtr(part.URL)}}
		case domainmessage.ContentPartVideoURL:
			p.Type = schema.ChatMessagePartTypeVideoURL
			p.Video = &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: stringPtr(part.URL)}}
		case domainmessage.ContentPartFileURL:
			p.Type = schema.ChatMessagePartTypeFileURL
			p.File = &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: stringPtr(part.URL)}}
		}
		result = append(result, p)
	}
	return result
}

func adaptPartsFromRunner(parts []schema.MessageInputPart) []domainmessage.ContentPart {
	result := make([]domainmessage.ContentPart, 0, len(parts))
	for _, part := range parts {
		p := domainmessage.ContentPart{Text: part.Text}
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			p.Type = domainmessage.ContentPartText
		case schema.ChatMessagePartTypeImageURL:
			p.Type = domainmessage.ContentPartImageURL
			if part.Image != nil {
				if part.Image.URL != nil {
					p.URL = *part.Image.URL
				}
				if part.Image.Base64Data != nil {
					p.Base64Data = *part.Image.Base64Data
				}
				p.MIMEType = part.Image.MIMEType
				p.Detail = string(part.Image.Detail)
			}
		case schema.ChatMessagePartTypeAudioURL:
			p.Type = domainmessage.ContentPartAudioURL
			if part.Audio != nil && part.Audio.URL != nil {
				p.URL = *part.Audio.URL
			}
		case schema.ChatMessagePartTypeVideoURL:
			p.Type = domainmessage.ContentPartVideoURL
			if part.Video != nil && part.Video.URL != nil {
				p.URL = *part.Video.URL
			}
		case schema.ChatMessagePartTypeFileURL:
			p.Type = domainmessage.ContentPartFileURL
			if part.File != nil && part.File.URL != nil {
				p.URL = *part.File.URL
			}
		}
		result = append(result, p)
	}
	return result
}

func adaptOutputPartsForRunner(parts []domainmessage.ContentPart) []schema.MessageOutputPart {
	result := make([]schema.MessageOutputPart, 0, len(parts))
	for _, part := range parts {
		p := schema.MessageOutputPart{Type: schema.ChatMessagePartType(part.Type), Text: part.Text}
		switch part.Type {
		case domainmessage.ContentPartImageURL:
			p.Image = &schema.MessageOutputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:        stringPtr(part.URL),
					Base64Data: stringPtr(part.Base64Data),
					MIMEType:   part.MIMEType,
				},
			}
		case domainmessage.ContentPartAudioURL:
			p.Audio = &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{URL: stringPtr(part.URL), Base64Data: stringPtr(part.Base64Data), MIMEType: part.MIMEType}}
		case domainmessage.ContentPartVideoURL:
			p.Video = &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: stringPtr(part.URL), Base64Data: stringPtr(part.Base64Data), MIMEType: part.MIMEType}}
		case domainmessage.ContentPartFileURL:
			p.Extra = map[string]any{"url": part.URL, "mime_type": part.MIMEType}
		}
		result = append(result, p)
	}
	return result
}

func adaptOutputPartsFromRunner(parts []schema.MessageOutputPart) []domainmessage.ContentPart {
	result := make([]domainmessage.ContentPart, 0, len(parts))
	for _, part := range parts {
		p := domainmessage.ContentPart{Type: domainmessage.ContentPartType(part.Type), Text: part.Text}
		switch part.Type {
		case schema.ChatMessagePartTypeImageURL:
			p.Type = domainmessage.ContentPartImageURL
			if part.Image != nil {
				if part.Image.URL != nil {
					p.URL = *part.Image.URL
				}
				if part.Image.Base64Data != nil {
					p.Base64Data = *part.Image.Base64Data
				}
				p.MIMEType = part.Image.MIMEType
			}
		case schema.ChatMessagePartTypeAudioURL:
			p.Type = domainmessage.ContentPartAudioURL
			if part.Audio != nil {
				if part.Audio.URL != nil {
					p.URL = *part.Audio.URL
				}
				if part.Audio.Base64Data != nil {
					p.Base64Data = *part.Audio.Base64Data
				}
				p.MIMEType = part.Audio.MIMEType
			}
		case schema.ChatMessagePartTypeVideoURL:
			p.Type = domainmessage.ContentPartVideoURL
			if part.Video != nil {
				if part.Video.URL != nil {
					p.URL = *part.Video.URL
				}
				if part.Video.Base64Data != nil {
					p.Base64Data = *part.Video.Base64Data
				}
				p.MIMEType = part.Video.MIMEType
			}
		case schema.ChatMessagePartTypeFileURL:
			p.Type = domainmessage.ContentPartFileURL
			if part.Extra != nil {
				if url, ok := part.Extra["url"].(string); ok {
					p.URL = url
				}
				if mimeType, ok := part.Extra["mime_type"].(string); ok {
					p.MIMEType = mimeType
				}
			}
		}
		result = append(result, p)
	}
	return result
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

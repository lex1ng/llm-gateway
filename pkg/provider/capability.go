package provider

import (
	"reflect"
)

// Capability represents a provider capability.
type Capability string

const (
	// --- Interface-level capabilities (auto-detected via reflection) ---
	CapChat      Capability = "chat"
	CapResponses Capability = "responses" // OpenAI Responses API
	CapEmbed     Capability = "embedding"
	CapImageGen  Capability = "image_gen"
	CapVideoGen  Capability = "video_gen"
	CapTTS       Capability = "tts"
	CapSTT       Capability = "stt"
	CapAgent     Capability = "agent"
	CapWorkflow  Capability = "workflow"

	// --- Feature-level capabilities (declared by Provider.Supports()) ---
	CapStream    Capability = "stream"
	CapTools     Capability = "tools"
	CapVision    Capability = "vision"
	CapJSONMode  Capability = "json_mode"
	CapReasoning Capability = "reasoning"
)

// capInterfaceMap maps interface-level capabilities to their corresponding interfaces.
// Used by Registry to auto-detect capabilities via reflection.
var capInterfaceMap = map[Capability]reflect.Type{
	CapChat:      reflect.TypeFor[ChatProvider](),
	CapResponses: reflect.TypeFor[ResponsesProvider](),
	CapEmbed:     reflect.TypeFor[EmbeddingProvider](),
	CapImageGen:  reflect.TypeFor[ImageGenProvider](),
	CapVideoGen:  reflect.TypeFor[VideoGenProvider](),
	CapTTS:       reflect.TypeFor[TTSProvider](),
	CapSTT:       reflect.TypeFor[STTProvider](),
	CapAgent:     reflect.TypeFor[AgentProvider](),
	CapWorkflow:  reflect.TypeFor[WorkflowProvider](),
}

// InterfaceCapabilities returns the list of interface-level capabilities.
func InterfaceCapabilities() []Capability {
	caps := make([]Capability, 0, len(capInterfaceMap))
	for cap := range capInterfaceMap {
		caps = append(caps, cap)
	}
	return caps
}

// IsInterfaceCapability returns true if the capability is interface-level.
func IsInterfaceCapability(cap Capability) bool {
	_, ok := capInterfaceMap[cap]
	return ok
}

// detectCapabilities checks which capability interfaces a provider implements.
func detectCapabilities(p Provider) []Capability {
	var caps []Capability
	pType := reflect.TypeOf(p)

	for cap, iface := range capInterfaceMap {
		if pType.Implements(iface) {
			caps = append(caps, cap)
		}
	}

	return caps
}

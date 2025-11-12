package ai

import "github.com/doeshing/shai-go/internal/ports"

func emitStream(req ports.ProviderRequest, content string) {
	if req.Stream && req.StreamWriter != nil {
		req.StreamWriter.WriteChunk(content)
	}
}

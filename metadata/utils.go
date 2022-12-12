package metadata

import "github.com/stability-ai/api-interfaces/gooseai/generation"

// RemoveBinaryData removes binary data from a request, which is useful for
// logging or printing output.
func RemoveBinaryData(rq *generation.Request) {
	for _, p := range rq.Prompt {
		artifact := p.GetArtifact()
		if artifact != nil {
		}
		if p.GetArtifact() != nil {
			if artifact.GetBinary() != nil {
				artifact.Data = nil
			}
			tensor := artifact.GetTensor()
			if tensor != nil {
				tensor.Data = nil
			}
		}
	}
}

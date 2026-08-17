package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

func MaskPublicUpstreamTopologyError(err *types.NewAPIError) (*types.NewAPIError, bool) {
	if err == nil || !newAPIErrorContainsUpstreamTopology(err) {
		return err, false
	}

	statusCode := err.StatusCode
	if statusCode < http.StatusBadRequest || statusCode > 599 {
		statusCode = http.StatusServiceUnavailable
	}
	return types.NewErrorWithStatusCode(
		errors.New(common.PublicPoolChannelUnavailableMessage),
		types.ErrorCodePoolChannelUnavailable,
		statusCode,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
	), true
}

func newAPIErrorContainsUpstreamTopology(err *types.NewAPIError) bool {
	candidates := []string{
		err.Error(),
		string(err.GetErrorType()),
		string(err.GetErrorCode()),
		string(err.Metadata),
	}
	switch relayErr := err.RelayError.(type) {
	case types.OpenAIError:
		candidates = append(candidates, relayErr.Message, relayErr.Type, fmt.Sprint(relayErr.Code), string(relayErr.Metadata))
	case *types.OpenAIError:
		if relayErr != nil {
			candidates = append(candidates, relayErr.Message, relayErr.Type, fmt.Sprint(relayErr.Code), string(relayErr.Metadata))
		}
	case types.ClaudeError:
		candidates = append(candidates, relayErr.Message, relayErr.Type)
	case *types.ClaudeError:
		if relayErr != nil {
			candidates = append(candidates, relayErr.Message, relayErr.Type)
		}
	}
	for _, candidate := range candidates {
		if common.ContainsUpstreamTopologyDetail(candidate) {
			return true
		}
	}
	return false
}

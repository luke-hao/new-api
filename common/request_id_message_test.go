package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripRequestIDAnnotations(t *testing.T) {
	message := "Invalid signature (request id: origin) (request id: gateway)"
	require.Equal(t, "Invalid signature", StripRequestIDAnnotations(message))
	require.Equal(t, "plain error", StripRequestIDAnnotations("plain error"))
}

func TestMessageWithRequestIdKeepsOnlyLocalID(t *testing.T) {
	message := "Invalid signature (request id: origin) (request id: gateway)"
	require.Equal(t, "Invalid signature (request id: local)", MessageWithRequestId(message, "local"))
}

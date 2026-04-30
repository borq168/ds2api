package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
)

func TestPostJSONWithStatusDoesNotFallbackAfterContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var fallbackCalled bool
	client := &Client{}
	_, _, err := client.postJSONWithStatus(
		ctx,
		failingDoer{err: context.Canceled},
		doerFunc(func(req *http.Request) (*http.Response, error) {
			fallbackCalled = true
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: req}, nil
		}),
		"https://example.com/api",
		nil,
		map[string]any{"foo": "bar"},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if fallbackCalled {
		t.Fatal("did not expect fallback request after context cancellation")
	}
}

func TestStreamPostDoesNotFallbackAfterContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var fallbackCalled bool
	client := &Client{
		fallbackS: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			fallbackCalled = true
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: [DONE]\n")), Request: req}, nil
		})},
	}
	_, err := client.streamPost(ctx, failingDoer{err: context.Canceled}, "https://example.com/api", nil, map[string]any{"foo": "bar"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if fallbackCalled {
		t.Fatal("did not expect fallback stream request after context cancellation")
	}
}

func TestCallCompletionReturnsCanceledWithoutAttemptingRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var streamCalls int
	client := &Client{
		stream: doerFunc(func(req *http.Request) (*http.Response, error) {
			streamCalls++
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: [DONE]\n")), Request: req}, nil
		}),
		fallbackS:  &http.Client{},
		maxRetries: 3,
	}
	_, err := client.CallCompletion(ctx, &auth.RequestAuth{DeepSeekToken: "token", TriedAccounts: map[string]bool{}}, map[string]any{"prompt": "hi"}, "pow", 3)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if streamCalls != 0 {
		t.Fatalf("expected no request attempts after context cancellation, got %d", streamCalls)
	}
}

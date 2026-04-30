package chat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

type canceledRetryDSStub struct {
	getPowCalls     int
	completionCalls int
}

func (m *canceledRetryDSStub) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "session-id", nil
}

func (m *canceledRetryDSStub) GetPow(ctx context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	m.getPowCalls++
	return "", ctx.Err()
}

func (m *canceledRetryDSStub) UploadFile(_ context.Context, _ *auth.RequestAuth, _ dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	return nil, errors.New("unexpected upload")
}

func (m *canceledRetryDSStub) CallCompletion(ctx context.Context, _ *auth.RequestAuth, _ map[string]any, _ string, _ int) (*http.Response, error) {
	m.completionCalls++
	return nil, ctx.Err()
}

func (m *canceledRetryDSStub) DeleteSessionForToken(_ context.Context, _ string, _ string) (*dsclient.DeleteSessionResult, error) {
	return &dsclient.DeleteSessionResult{Success: true}, nil
}

func (m *canceledRetryDSStub) DeleteAllSessionsForToken(_ context.Context, _ string) error {
	return nil
}

func TestHandleStreamWithRetryDoesNotRetryAfterRequestContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ds := &canceledRetryDSStub{}
	h := &Handler{DS: ds}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	resp := makeOpenAISSEHTTPResponse(
		`data: {"response_message_id":2,"p":"response/thinking_content","v":"plan"}`,
		"data: [DONE]",
	)

	h.handleStreamWithRetry(
		rec,
		req,
		&auth.RequestAuth{DeepSeekToken: "token", TriedAccounts: map[string]bool{}},
		resp,
		map[string]any{"chat_session_id": "session-id", "prompt": "prompt"},
		"pow",
		"cid-cancel",
		"deepseek-v4-pro",
		"prompt",
		true,
		false,
		nil,
		nil,
		nil,
	)

	if ds.getPowCalls != 0 || ds.completionCalls != 0 {
		t.Fatalf("expected no synthetic retry after canceled context, got getPow=%d completion=%d", ds.getPowCalls, ds.completionCalls)
	}
	if strings.Contains(rec.Body.String(), "Failed to get completion") {
		t.Fatalf("did not expect retry failure frame after cancellation, body=%s", rec.Body.String())
	}
}

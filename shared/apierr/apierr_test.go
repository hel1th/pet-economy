package apierr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/apierr"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type envelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Field     string `json:"field"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()

	var body envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return body
}

func TestWriteMapsErrorsToStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{
			name:       "invalid error becomes validation",
			err:        domainerr.NewInvalid("title", "field is required"),
			wantStatus: http.StatusBadRequest,
			wantCode:   apierr.CodeValidation,
			wantField:  "title",
		},
		{
			name:       "wrapped invalid error is unwrapped",
			err:        fmt.Errorf("update: %w", domainerr.NewInvalid("price", "must be positive")),
			wantStatus: http.StatusBadRequest,
			wantCode:   apierr.CodeValidation,
			wantField:  "price",
		},
		{
			name:       "conflict struct error",
			err:        domainerr.NewConflict("already published"),
			wantStatus: http.StatusConflict,
			wantCode:   apierr.CodeConflict,
		},
		{
			name:       "sentinel not found",
			err:        fmt.Errorf("item not found: %w", domainerr.ErrNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   apierr.CodeNotFound,
		},
		{
			name:       "sentinel forbidden",
			err:        domainerr.ErrForbidden,
			wantStatus: http.StatusForbidden,
			wantCode:   apierr.CodeForbidden,
		},
		{
			name:       "sentinel unauthorized",
			err:        domainerr.ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
			wantCode:   apierr.CodeUnauthorized,
		},
		{
			name:       "sentinel conflict",
			err:        fmt.Errorf("state: %w", domainerr.ErrConflict),
			wantStatus: http.StatusConflict,
			wantCode:   apierr.CodeConflict,
		},
		{
			name:       "unknown error hides details",
			err:        errors.New("connection reset by peer"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   apierr.CodeInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/resource", http.NoBody)

			apierr.Write(rec, req, tc.err)

			require.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

			body := decode(t, rec)
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantField, body.Error.Field)
			assert.NotEmpty(t, body.Error.Message)

			if tc.wantStatus == http.StatusInternalServerError {
				assert.Equal(t, "internal server error", body.Error.Message)
				assert.NotContains(t, body.Error.Message, "connection reset")
			}
		})
	}
}

func TestWriteHidesInternalChainForClientErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantMessage string
	}{
		{
			name:        "not found hides chain",
			err:         fmt.Errorf("load item: %w", domainerr.ErrNotFound),
			wantMessage: apierr.MessageNotFound,
		},
		{
			name:        "forbidden hides chain",
			err:         fmt.Errorf("authorize actor: %w", domainerr.ErrForbidden),
			wantMessage: apierr.MessageForbidden,
		},
		{
			name:        "unauthorized hides chain",
			err:         fmt.Errorf("parse bearer token: %w", domainerr.ErrUnauthorized),
			wantMessage: apierr.MessageUnauthorized,
		},
		{
			name:        "conflict sentinel hides chain",
			err:         fmt.Errorf("persist item status: %w", domainerr.ErrConflict),
			wantMessage: apierr.MessageConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			apierr.Write(rec, httptest.NewRequest(http.MethodGet, "/resource", http.NoBody), tc.err)

			body := decode(t, rec)
			assert.Equal(t, tc.wantMessage, body.Error.Message)
			assert.NotContains(t, body.Error.Message, ":")
		})
	}
}

func TestWritePrefersInvalidOverSentinel(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resource", http.NoBody)

	apierr.Write(rec, req, fmt.Errorf("%w: %w", domainerr.NewInvalid("id", "bad"), domainerr.ErrNotFound))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, apierr.CodeValidation, decode(t, rec).Error.Code)
}

func TestWriteBadRequest(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resource", http.NoBody)

	apierr.WriteBadRequest(rec, req, "malformed request body")

	require.Equal(t, http.StatusBadRequest, rec.Code)

	body := decode(t, rec)
	assert.Equal(t, apierr.CodeBadRequest, body.Error.Code)
	assert.Equal(t, "malformed request body", body.Error.Message)
}

func TestConflictErrorMatchesSentinel(t *testing.T) {
	t.Parallel()

	err := domainerr.NewConflict("duplicate")

	require.ErrorIs(t, err, domainerr.ErrConflict)
	assert.Equal(t, "duplicate", err.Error())
	assert.Equal(t, "field: message", domainerr.NewInvalid("field", "message").Error())
}

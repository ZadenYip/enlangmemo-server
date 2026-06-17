package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeUserStore struct {
	err error
}

func (store fakeUserStore) CreateUser(ctx context.Context, name string, passwordHash string) error {
	return store.err
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		store      userStore
		body       string
		wantStatus int
	}{
		{
			name:       "name too long",
			store:      fakeUserStore{},
			body:       `{"name":"abcdefghijklmnopq","password":"password"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "user already exists",
			store:      fakeUserStore{err: errUserAlreadyExists},
			body:       `{"name":"alice","password":"password"}`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "store error",
			store:      fakeUserStore{err: errors.New("store error")},
			body:       `{"name":"alice","password":"password"}`,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "created",
			store:      fakeUserStore{},
			body:       `{"name":"alice","password":"password"}`,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &Server{users: tt.store}
			req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			srv.Register(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

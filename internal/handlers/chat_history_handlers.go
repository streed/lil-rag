package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"lil-rag/pkg/chathistory"
)

// ChatSessions handles GET requests for listing all chat sessions
func (h *Handler) ChatSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.chatHistory == nil {
			h.writeError(w, http.StatusServiceUnavailable, "chat history not available", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		sessions, err := h.chatHistory.GetSessions(ctx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to get chat sessions", err.Error())
			return
		}

		// Ensure we always return an empty array instead of null
		if sessions == nil {
			sessions = []chathistory.ChatSession{}
		}

		w.Header().Set("Content-Type", "application/json")
		response := ChatSessionsResponse{Sessions: sessions}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode response", err.Error())
		}
	}
}

// CreateChatSession handles POST requests for creating new chat sessions
func (h *Handler) CreateChatSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.chatHistory == nil {
			h.writeError(w, http.StatusServiceUnavailable, "chat history not available", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		session, err := h.chatHistory.CreateSession(ctx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to create chat session", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := CreateSessionResponse{Session: session}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode response", err.Error())
		}
	}
}

// ChatHistory handles GET requests for retrieving chat history for a specific session
func (h *Handler) ChatHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.chatHistory == nil {
			h.writeError(w, http.StatusServiceUnavailable, "chat history not available", "")
			return
		}

		// Extract session ID from URL path /api/chat/history/{session_id}
		path := strings.TrimPrefix(r.URL.Path, "/api/chat/history/")
		sessionID := strings.TrimSuffix(path, "/")
		if sessionID == "" {
			h.writeError(w, http.StatusBadRequest, "session ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		chatContext, err := h.chatHistory.GetChatContext(ctx, sessionID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "chat session not found", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := ChatHistoryResponse{Messages: chatContext.Messages}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode response", err.Error())
		}
	}
}

// DeleteChatSession handles DELETE requests for deleting chat sessions
func (h *Handler) DeleteChatSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.chatHistory == nil {
			h.writeError(w, http.StatusServiceUnavailable, "chat history not available", "")
			return
		}

		// Extract session ID from URL path /api/chat/sessions/{session_id}
		path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		sessionID := strings.TrimSuffix(path, "/")
		if sessionID == "" {
			h.writeError(w, http.StatusBadRequest, "session ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		err := h.chatHistory.DeleteSession(ctx, sessionID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to delete chat session", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"status":  "success",
			"message": "Chat session deleted successfully",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode response", err.Error())
		}
	}
}

// UpdateChatSessionTitle handles PUT requests for updating chat session titles
func (h *Handler) UpdateChatSessionTitle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.chatHistory == nil {
			h.writeError(w, http.StatusServiceUnavailable, "chat history not available", "")
			return
		}

		// Extract session ID from URL path /api/chat/sessions/{session_id}/title
		path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		path = strings.TrimSuffix(path, "/title")
		sessionID := strings.TrimSuffix(path, "/")
		if sessionID == "" {
			h.writeError(w, http.StatusBadRequest, "session ID required", "")
			return
		}

		var req struct {
			Title string `json:"title"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid JSON", err.Error())
			return
		}

		if strings.TrimSpace(req.Title) == "" {
			h.writeError(w, http.StatusBadRequest, "title cannot be empty", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		err := h.chatHistory.UpdateSessionTitle(ctx, sessionID, req.Title)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to update session title", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"status":  "success",
			"message": "Chat session title updated successfully",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode response", err.Error())
		}
	}
}

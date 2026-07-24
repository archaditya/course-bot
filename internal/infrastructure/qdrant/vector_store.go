// Package qdrant implements provider.VectorStore against the Qdrant REST API.
// See docs/07-storage.md. Only Go Workers ever call Upsert; the AI Service
// calls Qdrant directly for hybrid search at query time — so this package
// only needs Upsert, Search, and DeleteByCourse.
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"archadilm/internal/domain/provider"
)

const collectionName = "course_chunks"

// Store wraps the Qdrant HTTP API.
type Store struct {
	baseURL    string
	apiKey     string
	collection string
	http       *http.Client
}

// NewStore returns a Store pointing at qdrantURL (e.g. "http://localhost:6333").
// apiKey is optional (empty for local Qdrant without auth).
func NewStore(qdrantURL, apiKey string) (*Store, error) {
	s := &Store{
		baseURL:    strings.TrimRight(qdrantURL, "/"),
		apiKey:     apiKey,
		collection: collectionName,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
	if err := s.ensureCollection(context.Background()); err != nil {
		return nil, fmt.Errorf("qdrant: ensure collection: %w", err)
	}
	return s, nil
}

// Upsert writes vectors and their minimal payload (chunk_id, course_id,
// start_timestamp) to Qdrant. Called by Embedding Worker only.
// Qdrant is treated as a rebuildable derived index — see ADR-002.
func (s *Store) Upsert(ctx context.Context, points []provider.VectorPoint) error {
	type qdrantPoint struct {
		ID      string                 `json:"id"`
		Vector  []float32              `json:"vector"`
		Payload map[string]interface{} `json:"payload"`
	}
	pts := make([]qdrantPoint, len(points))
	for i, p := range points {
		payload := map[string]interface{}{
			"chunk_id":  p.ChunkID,
			"course_id": p.CourseID,
		}
		if p.StartTimestamp != nil {
			payload["start_timestamp"] = *p.StartTimestamp
		}
		pts[i] = qdrantPoint{
			ID:      p.ChunkID,
			Vector:  []float32(p.Vector),
			Payload: payload,
		}
	}
	body, _ := json.Marshal(map[string]any{"points": pts})
	return s.put(ctx, "/collections/"+s.collection+"/points?wait=true", body)
}

// Search performs a vector nearest-neighbour search filtered by course_id.
func (s *Store) Search(ctx context.Context, courseID string, query provider.Vector, topK int) ([]provider.VectorSearchResult, error) {
	body, _ := json.Marshal(map[string]any{
		"vector": []float32(query),
		"filter": map[string]any{
			"must": []map[string]any{
				{"key": "course_id", "match": map[string]any{"value": courseID}},
			},
		},
		"limit":        topK,
		"with_payload": false,
	})
	var resp struct {
		Result []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"result"`
	}
	if err := s.postJSON(ctx, "/collections/"+s.collection+"/points/search", body, &resp); err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}
	results := make([]provider.VectorSearchResult, len(resp.Result))
	for i, r := range resp.Result {
		results[i] = provider.VectorSearchResult{ChunkID: r.ID, Score: r.Score}
	}
	return results, nil
}

// DeleteByCourse removes all vectors whose course_id payload matches. Called
// when a course is deleted or re-indexed.
func (s *Store) DeleteByCourse(ctx context.Context, courseID string) error {
	body, _ := json.Marshal(map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{"key": "course_id", "match": map[string]any{"value": courseID}},
			},
		},
	})
	return s.postNoResp(ctx, "/collections/"+s.collection+"/points/delete?wait=true", body)
}

// ensureCollection creates the collection if it doesn't exist. Uses cosine
// distance and 1536 dimensions (text-embedding-3-small). If the embedding
// model changes, the collection must be rebuilt — see
// docs/04-indexing-pipeline.md#versioning-strategy.
func (s *Store) ensureCollection(ctx context.Context) error {
	// Check existence
	resp, err := s.doRequest(ctx, http.MethodGet, "/collections/"+s.collection, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil // already exists
	}

	// Create
	body, _ := json.Marshal(map[string]any{
		"vectors": map[string]any{
			"size":     1536,
			"distance": "Cosine",
		},
	})
	return s.put(ctx, "/collections/"+s.collection, body)
}

func (s *Store) put(ctx context.Context, path string, body []byte) error {
	resp, err := s.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant: PUT %s: status %d: %s", path, resp.StatusCode, string(raw))
	}
	return nil
}

func (s *Store) postJSON(ctx context.Context, path string, body []byte, out any) error {
	resp, err := s.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant: POST %s: status %d: %s", path, resp.StatusCode, string(raw))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Store) postNoResp(ctx context.Context, path string, body []byte) error {
	resp, err := s.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant: POST %s: status %d: %s", path, resp.StatusCode, string(raw))
	}
	return nil
}

// Ping checks that Qdrant is reachable and the collection exists. Used by
// the API's /healthz check.
func (s *Store) Ping(ctx context.Context) error {
	resp, err := s.doRequest(ctx, http.MethodGet, "/collections/"+s.collection, nil)
	if err != nil {
		return fmt.Errorf("qdrant: ping: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant: ping: status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func (s *Store) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.apiKey != "" {
		req.Header.Set("api-key", s.apiKey)
	}
	return s.http.Do(req)
}
